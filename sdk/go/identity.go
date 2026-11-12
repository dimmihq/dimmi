package dimmi

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

/*
User is a signed-in user of your app, as the messenger should know them.
ID is required. An email counts as verified only when EmailVerified is
set. Attributes are flat values under snake_case keys.
*/
type User struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	Phone         string
	PhoneVerified bool
	AvatarURL     string
	Locale        string
	Timezone      string
	SignedUpAt    time.Time
	Company       *Company
	Attributes    map[string]any
}

// Company is the company a user belongs to; ID is required.
type Company struct {
	ID         string
	Name       string
	Domain     string
	Attributes map[string]any
}

/*
IdentityOptions say how to sign. Without a private key the shared secret
signs HS256; a PKCS #8 PEM private key signs ES256, RS256 or EdDSA and
needs the key id dimmi showed for its public key.
*/
type IdentityOptions struct {
	// Secret is the shared secret, dmi_idsec_..., DIMMI_IDENTITY_SECRET by default.
	Secret string
	// KeyID is the key id dimmi shows for the secret or key, DIMMI_IDENTITY_KID by default.
	KeyID string
	// PrivateKey is a PKCS #8 PEM private key, DIMMI_IDENTITY_PRIVATE_KEY by default.
	PrivateKey string
	// Algorithm pins the private key's algorithm: ES256, RS256 or EdDSA.
	Algorithm string
	// ExpiresIn is how long the token lives: 10 minutes unless set, an hour at most.
	ExpiresIn time.Duration
	// Audience is "dimmi", a messenger app id or a workspace id.
	Audience string
	// Nonce makes the token work only once.
	Nonce string
	// Now is the signing time, time.Now by default.
	Now func() time.Time
}

/*
IdentityError is a user or key that cannot become a token, with a stable
code and the fix in its message.
*/
type IdentityError struct {
	Code    string
	Message string
}

func (e *IdentityError) Error() string { return "[dimmi] " + e.Code + ": " + e.Message }

func identityErr(code, msg string) error { return &IdentityError{Code: code, Message: msg} }

var attributeKey = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

/*
Claims turns a user into token claims, the same rules @dimmi/server
applies: empty fields are left out and attributes are checked.
*/
func Claims(u User) (map[string]any, error) {
	sub := strings.TrimSpace(u.ID)
	if sub == "" || len(u.ID) > 255 {
		return nil, identityErr("identify_user_invalid", "the user needs an id: a string or number, 1 to 255 long.")
	}
	claims := map[string]any{"sub": u.ID}
	set(claims, "email", u.Email)
	if u.Email != "" && u.EmailVerified {
		claims["email_verified"] = true
	}
	set(claims, "name", u.Name)
	set(claims, "phone", u.Phone)
	if u.Phone != "" && u.PhoneVerified {
		claims["phone_verified"] = true
	}
	set(claims, "avatar_url", u.AvatarURL)
	set(claims, "locale", u.Locale)
	set(claims, "timezone", u.Timezone)
	attrs, err := checkAttributes(u.Attributes)
	if err != nil {
		return nil, err
	}
	if !u.SignedUpAt.IsZero() {
		attrs["signed_up_at"] = u.SignedUpAt.Unix()
	}
	if len(attrs) > 0 {
		claims["custom_attributes"] = attrs
	}
	if u.Company != nil {
		if u.Company.ID == "" {
			return nil, identityErr("identify_user_invalid", "company needs an id.")
		}
		company := map[string]any{"id": u.Company.ID}
		set(company, "name", u.Company.Name)
		set(company, "domain", u.Company.Domain)
		if len(u.Company.Attributes) > 0 {
			ca, err := checkAttributes(u.Company.Attributes)
			if err != nil {
				return nil, err
			}
			company["custom_attributes"] = ca
		}
		claims["company"] = company
	}
	return claims, nil
}

func set(m map[string]any, name, value string) {
	if value != "" {
		m[name] = value
	}
}

func checkAttributes(in map[string]any) (map[string]any, error) {
	if len(in) > 100 {
		return nil, identityErr("attribute_limit", "send at most 100 attributes; update others through the API.")
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if !attributeKey.MatchString(k) {
			return nil, identityErr("attribute_key_invalid", fmt.Sprintf("use snake_case keys such as plan_name, not %s.", k))
		}
		switch v.(type) {
		case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		default:
			return nil, identityErr("attribute_value_invalid", fmt.Sprintf("%s must be a string, number, boolean or null.", k))
		}
		out[k] = v
	}
	return out, nil
}

/*
SignIdentityToken signs a short-lived token naming the user, for the
messenger to verify. Hand it to the page with MessengerSnippet or answer
it from a token route with TokenHandler.
*/
func SignIdentityToken(u User, opts IdentityOptions) (string, error) {
	expires := opts.ExpiresIn
	if expires == 0 {
		expires = 10 * time.Minute
	}
	if expires < time.Second || expires > time.Hour {
		return "", identityErr("expires_in_invalid", "ExpiresIn is 1 second to 1 hour.")
	}
	claims, err := Claims(u)
	if err != nil {
		return "", err
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	iat := now().Unix()
	claims["iat"] = iat
	claims["exp"] = iat + int64(expires/time.Second)
	if opts.Audience != "" {
		claims["aud"] = opts.Audience
	}
	if opts.Nonce != "" {
		claims["nonce"] = opts.Nonce
	}
	method, key, kid, err := signingKey(opts)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(method, jwt.MapClaims(claims))
	if kid != "" {
		token.Header["kid"] = kid
	}
	return token.SignedString(key)
}

func signingKey(opts IdentityOptions) (jwt.SigningMethod, any, string, error) {
	kid := orEnv(opts.KeyID, "DIMMI_IDENTITY_KID")
	if pemKey := orEnv(opts.PrivateKey, "DIMMI_IDENTITY_PRIVATE_KEY"); pemKey != "" {
		if kid == "" {
			return nil, nil, "", identityErr("kid_required", "set DIMMI_IDENTITY_KID to the key id dimmi shows for your public key.")
		}
		method, key, err := parsePrivateKey(pemKey, opts.Algorithm)
		return method, key, kid, err
	}
	secret := strings.TrimSpace(orEnv(opts.Secret, "DIMMI_IDENTITY_SECRET"))
	if secret == "" {
		return nil, nil, "", identityErr("secret_missing", "set DIMMI_IDENTITY_SECRET to the secret from Settings > Identity verification.")
	}
	// The whole dmi_idsec_ string is the HMAC key, never decoded.
	return jwt.SigningMethodHS256, []byte(secret), kid, nil
}

func parsePrivateKey(pemKey, algorithm string) (jwt.SigningMethod, any, error) {
	invalid := identityErr("private_key_invalid", "the private key must be PKCS #8 PEM: Ed25519, P-256 or RSA.")
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, nil, invalid
	}
	if strings.Contains(block.Type, "PUBLIC KEY") {
		return nil, nil, identityErr("private_key_invalid", "that is a public key; sign with the private key.")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, invalid
	}
	var method jwt.SigningMethod
	switch k := parsed.(type) {
	case ed25519.PrivateKey:
		method = jwt.SigningMethodEdDSA
	case *ecdsa.PrivateKey:
		if k.Curve != elliptic.P256() {
			return nil, nil, invalid
		}
		method = jwt.SigningMethodES256
	case *rsa.PrivateKey:
		method = jwt.SigningMethodRS256
	default:
		return nil, nil, invalid
	}
	if algorithm != "" && algorithm != method.Alg() {
		return nil, nil, invalid
	}
	return method, parsed, nil
}

func orEnv(v, name string) string {
	if v != "" {
		return v
	}
	return os.Getenv(name)
}

/*
SnippetOptions place the messenger on a page.
*/
type SnippetOptions struct {
	// AppID is the messenger's app id, wgt_..., from Settings > Messenger.
	AppID string
	// Host is where dimmi runs: https://app.trydimmi.com or your own address.
	Host string
	// Token is a signed identity token for the signed-in user; empty for visitors.
	Token string
	// IdentityURL is a route on your site answering {"token": ...}, instead of Token.
	IdentityURL string
	// Nonce is the script nonce under a strict Content-Security-Policy.
	Nonce string
}

/*
MessengerSnippet is the HTML that loads the messenger, identifying the
user when a token or identity URL is given.
*/
func MessengerSnippet(o SnippetOptions) string {
	nonce := ""
	if o.Nonce != "" {
		nonce = ` nonce="` + html.EscapeString(o.Nonce) + `"`
	}
	var b strings.Builder
	b.WriteString("<script" + nonce + ">window.Dimmi=window.Dimmi||function(){(Dimmi.q=Dimmi.q||[]).push([].slice.call(arguments))};")
	if o.Token != "" {
		raw, _ := json.Marshal(o.Token)
		// json.Marshal escapes <, > and & so the token cannot close the script.
		b.WriteString(`Dimmi("identify",function(){return ` + string(raw) + `});`)
	}
	b.WriteString("</script>\n")
	src := strings.TrimRight(o.Host, "/") + "/widget/v1/loader.js"
	b.WriteString(`<script async src="` + html.EscapeString(src) + `" data-app-id="` + html.EscapeString(o.AppID) + `"`)
	if o.IdentityURL != "" {
		b.WriteString(` data-identity-url="` + html.EscapeString(o.IdentityURL) + `"`)
	}
	b.WriteString(nonce + "></script>")
	return b.String()
}

/*
TokenHandler answers the messenger's identity URL: {"token": ...} for the
user identify returns, {"token": null} when nobody is signed in, never
cached.
*/
func TokenHandler(identify func(*http.Request) (*User, error), opts IdentityOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store, private")
		user, err := identify(r)
		if err != nil {
			fail(w, "identify_failed")
			return
		}
		var token *string
		if user != nil {
			signed, err := SignIdentityToken(*user, opts)
			if err != nil {
				var ie *IdentityError
				code := "sign_failed"
				if errors.As(err, &ie) {
					code = ie.Code
				}
				fail(w, code)
				return
			}
			token = &signed
		}
		_ = json.NewEncoder(w).Encode(map[string]*string{"token": token})
	})
}

func fail(w http.ResponseWriter, code string) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`{"token":null,"error":` + strconv.Quote(code) + `}`))
}
