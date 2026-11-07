/*
|--------------------------------------------------------------------------
| Connections
|--------------------------------------------------------------------------
|
| A connection's secret is sealed with the workspace's keys and bound to
| its own row, so a blob copied to another connection never opens. It
| is applied to a request only after every template is rendered, never
| visible to a template, a trace or the model. A base URL must use TLS
| unless the operator trusts the host, and may carry no credentials,
| query or fragment of its own.
|
*/

package actionkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AuthNone is a connection that sends no credential.
*/
const AuthNone = "none"

/*
secretMask stands in for a secret wherever a request is shown.
*/
const secretMask = "[secret]"

/*
AuthTypes lists the ways a connection may authenticate.
*/
func AuthTypes() []string {
	return []string{AuthNone, "bearer", "api_key_header", "api_key_query", "basic"}
}

/*
Credentials is a connection's opened secret: a token, or a username and
password for basic auth. It never prints its values.
*/
type Credentials struct {
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

/*
String hides the secret.
*/
func (Credentials) String() string { return "[redacted]" }

/*
GoString hides the secret.
*/
func (Credentials) GoString() string { return "[redacted]" }

/*
Format hides the secret whatever the verb.
*/
func (Credentials) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte("[redacted]")) }

/*
Hint is the tail of the secret a teammate may see to tell keys apart.
*/
func (c Credentials) Hint() *string {
	secret := c.Token
	if secret == "" {
		secret = c.Password
	}
	if len(secret) < 8 {
		return nil
	}
	hint := "…" + secret[len(secret)-4:]
	return &hint
}

/*
AuthConfig is a connection's non-secret auth setting: the header an API
key travels in, the query parameter, or a prefix other than Bearer.
*/
type AuthConfig struct {
	HeaderName string `json:"header_name,omitempty"`
	QueryParam string `json:"query_param,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
}

/*
credentialAAD names the column and row a secret belongs to.
*/
func credentialAAD(connectionID ids.UUID) []byte {
	return []byte("ai_action_connections.credentials_encrypted:" + connectionID.String())
}

/*
SealCredentials seals a connection's secret.
*/
func SealCredentials(ctx context.Context, keys *encryption.Keys, conn models.AiActionConnection,
	creds Credentials,
) (encryption.Sealed, error) {
	sealed, err := encryption.Seal(ctx, keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: conn.WorkspaceID.String()},
		AAD:   credentialAAD(conn.ID),
	}, creds)
	if err != nil {
		return nil, fmt.Errorf("seal action credentials: %w", err)
	}
	return sealed, nil
}

/*
OpenCredentials opens a connection's secret, empty when it has none.
*/
func OpenCredentials(ctx context.Context, keys *encryption.Keys, conn models.AiActionConnection) (Credentials, error) {
	if len(conn.CredentialsEncrypted) == 0 {
		return Credentials{}, nil
	}
	creds, err := encryption.Open[Credentials](ctx, keys, conn.CredentialsEncrypted, credentialAAD(conn.ID))
	if err != nil {
		return Credentials{}, fmt.Errorf("open action credentials: %w", err)
	}
	return creds, nil
}

/*
NormalizeBaseURL checks a base URL a credential may be sent to and
returns it in normal form: lowercase host, no trailing slash.
*/
func NormalizeBaseURL(raw string, client *httpclient.Client) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fault.New(fault.Invalid, "base_url_invalid",
			"the base url must be an absolute https url without credentials, query or fragment").OnField("base_url")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	switch u.Scheme {
	case "https":
	case "http":
		if client == nil || !client.AllowsPlaintext(hostPort(u)) {
			return "", exceptions.ErrActionHostBlocked.OnField("base_url")
		}
	default:
		return "", fault.New(fault.Invalid, "base_url_invalid", "the base url must use https").OnField("base_url")
	}
	if client != nil && client.CheckAddress(hostPort(u)) != nil {
		return "", exceptions.ErrActionHostBlocked.OnField("base_url")
	}
	return u.String(), nil
}

/*
hostPort is a URL's host with its port, the scheme's default when none
is written.
*/
func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	if u.Scheme == "http" {
		return u.Host + ":80"
	}
	return u.Host + ":443"
}

/*
CheckAuth checks an auth type and its setting.
*/
func CheckAuth(authType string, cfg AuthConfig) error {
	if !slices.Contains(AuthTypes(), authType) {
		return fault.New(fault.Invalid, "auth_type_invalid", "unknown auth type").OnField("auth_type")
	}
	switch authType {
	case "api_key_header":
		if !headerPattern().MatchString(cfg.HeaderName) || reservedHeader(cfg.HeaderName) &&
			!strings.EqualFold(cfg.HeaderName, "authorization") {
			return fault.New(fault.Invalid, "header_name_invalid", "name the header the key travels in").
				OnField("auth_config.header_name")
		}
	case "api_key_query":
		if !headerPattern().MatchString(cfg.QueryParam) {
			return fault.New(fault.Invalid, "query_param_invalid", "name the query parameter the key travels in").
				OnField("auth_config.query_param")
		}
	}
	return nil
}

/*
applied is how a connection's auth lands on a request: headers to add
and query parameters to set, each with the value shown in traces.
*/
type applied struct {
	headers fhttp.Header
	query   url.Values
}

/*
applyAuth turns the connection's auth and secret into what the request
carries.
*/
func applyAuth(conn models.AiActionConnection, creds Credentials) applied {
	out := applied{headers: fhttp.Header{}, query: url.Values{}}
	var cfg AuthConfig
	_ = json.Unmarshal(conn.AuthConfig, &cfg)
	switch conn.AuthType {
	case "bearer":
		prefix := cfg.Prefix
		if prefix == "" {
			prefix = "Bearer"
		}
		out.headers.Set("Authorization", prefix+" "+creds.Token)
	case "api_key_header":
		out.headers.Set(cfg.HeaderName, creds.Token)
	case "api_key_query":
		out.query.Set(cfg.QueryParam, creds.Token)
	case "basic":
		raw := creds.Username + ":" + creds.Password
		out.headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(raw)))
	}
	return out
}

/*
defaultHeaders reads the connection's static headers.
*/
func defaultHeaders(conn models.AiActionConnection) map[string]string {
	out := map[string]string{}
	_ = json.Unmarshal(conn.DefaultHeaders, &out)
	return out
}

/*
CheckHeaders checks a set of static header names and values.
*/
func CheckHeaders(headers map[string]string) error {
	if len(headers) > maxHeaders {
		return fault.New(fault.Invalid, "too_many_headers", "at most 10 headers").OnField("default_headers")
	}
	for name, value := range headers {
		if !headerPattern().MatchString(name) || reservedHeader(name) || strings.ContainsAny(value, "\r\n\x00") ||
			len(value) > 256 {
			return fault.New(fault.Invalid, "header_invalid", "header "+name+" may not be set").
				OnField("default_headers")
		}
	}
	return nil
}

/*
CheckVariables checks a connection's template variables.
*/
func CheckVariables(vars map[string]string) error {
	if len(vars) > 20 {
		return fault.New(fault.Invalid, "too_many_variables", "at most 20 variables").OnField("variables")
	}
	for k, v := range vars {
		if !variablePattern(k) || len(v) > 256 {
			return fault.New(fault.Invalid, "variable_invalid", "variable "+k+" is not valid").OnField("variables")
		}
	}
	return nil
}

/*
variablePattern reports whether a variable name is lowercase words.
*/
func variablePattern(name string) bool {
	if name == "" || len(name) > 40 {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
