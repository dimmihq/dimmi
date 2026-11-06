/*
|--------------------------------------------------------------------------
| LLM Credentials
|--------------------------------------------------------------------------
|
| A connection's secret is sealed with the workspace's keys and bound
| to its own row: the associated data names the column and the key id,
| so a sealed blob copied onto another connection never opens. Every
| place that seals or opens one goes through the slot here, so sealing
| and opening can never disagree. A base URL a credential is sent to
| must use TLS, unless the operator explicitly trusts the host.
|
*/

package managers

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Credentials is a connection's opened secret shape.
*/
type Credentials struct {
	APIKey       string            `json:"api_key,omitempty"`
	AccessToken  string            `json:"access_token,omitempty"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	IDToken      string            `json:"id_token,omitempty"`
	AccountID    string            `json:"account_id,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at,omitzero"`
	Scopes       []string          `json:"scopes,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

/*
CredentialSlot is where a connection's credentials are sealed: the
workspace's scope, bound to the key's row.
*/
func CredentialSlot(workspaceID, keyID ids.UUID) encryption.Slot {
	return encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: workspaceID.String()},
		AAD:   credentialAAD(keyID),
	}
}

/*
credentialAAD names the column and row a credential belongs to.
*/
func credentialAAD(keyID ids.UUID) []byte {
	return []byte("ai_provider_keys.credentials_encrypted:" + keyID.String())
}

/*
SealCredentials seals credentials for a key row, which must already
have its id.
*/
func SealCredentials(ctx context.Context, keys *encryption.Keys, key models.AiProviderKey,
	creds Credentials,
) (encryption.Sealed, error) {
	sealed, err := encryption.Seal(ctx, keys, CredentialSlot(key.WorkspaceID, key.ID), creds)
	if err != nil {
		return nil, fmt.Errorf("seal provider credentials: %w", err)
	}
	return sealed, nil
}

/*
OpenCredentials opens a key row's credentials, empty for managed and
none-auth connections.
*/
func OpenCredentials(ctx context.Context, keys *encryption.Keys, key models.AiProviderKey) (Credentials, error) {
	if len(key.CredentialsEncrypted) == 0 {
		return Credentials{}, nil
	}

	creds, err := encryption.Open[Credentials](ctx, keys, key.CredentialsEncrypted, credentialAAD(key.ID))
	if err != nil {
		return Credentials{}, fmt.Errorf("open provider credentials: %w", err)
	}
	return creds, nil
}

/*
CheckBaseURL refuses a connection endpoint a credential may not be
sent to: anything but an absolute http or https URL without user info,
query or fragment, and plain http to a host the operator did not
trust in the HTTP client's allow list or private-network setting.
*/
func (m *LLMManager) CheckBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fault.New(fault.Invalid, "base_url_invalid", "the base url must be an absolute http or https url")
	}

	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if m.http != nil && m.http.AllowsPlaintext(hostPort(u)) {
			return nil
		}
		return fault.New(fault.Invalid, "base_url_insecure", "the base url must use https")
	}
	return fault.New(fault.Invalid, "base_url_invalid", "the base url must be an absolute http or https url")
}

/*
hostPort is a URL's host with its port, the scheme's default when
none is written.
*/
func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	return u.Host + ":80"
}
