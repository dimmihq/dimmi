/*
|--------------------------------------------------------------------------
| Domain Connect
|--------------------------------------------------------------------------
|
| Some DNS hosts set records up themselves once the domain's owner agrees
| on the host's own page. The domain's TXT record names the host's API,
| the API says where its consent page is, and the host must already know
| the template; any failure along the way only means the records are
| added by hand, so every lookup is quick and every error just answers
| "not offered".
|
| The link to the consent page is signed with the template owner's key,
| whose public half the host reads from a TXT record, so nobody else can
| send a customer to apply our records with their values.
|
*/

package domainconnect

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
lookupTimeout bounds each call to a host's API.
*/
const lookupTimeout = 3 * time.Second

/*
ErrInvalidKey means the signing key is not a PEM encoded RSA key.
*/
var ErrInvalidKey = errors.New("domainconnect: invalid signing key")

/*
Template names a template: who published it and which service it sets
up.
*/
type Template struct {
	ProviderID string
	ServiceID  string
}

/*
Host is a DNS host that can apply the template to a domain: its name,
for the button, and the base of its consent page.
*/
type Host struct {
	Name   string
	SyncUX string
}

/*
Finder finds hosts through the resolver and the HTTP client.
*/
type Finder struct {
	Resolver dns.Resolver
	Client   *httpclient.Client
}

/*
Discover returns the host that can apply the template to the domain, and
whether there is one. Domain is the zone's root as the host knows it.
*/
func (f Finder) Discover(ctx context.Context, domain string, t Template) (Host, bool) {
	resolver, client := f.Resolver, f.Client

	texts, err := resolver.TXT(ctx, "_domainconnect."+domain)
	if err != nil || len(texts) == 0 {
		return Host{}, false
	}

	api := "https://" + strings.TrimSuffix(strings.TrimSpace(texts[0]), "/")

	resp, err := client.New().Timeout(lookupTimeout).Get(ctx, api+"/v2/"+url.PathEscape(domain)+"/settings")
	if err != nil || resp.Status != 200 {
		return Host{}, false
	}

	var found map[string]any
	if resp.JSON(&found) != nil || text(found, "urlSyncUX") == "" {
		return Host{}, false
	}

	base := strings.TrimSuffix(firstNonEmpty(text(found, "urlAPI"), api), "/")
	template := base + "/v2/domainTemplates/providers/" + url.PathEscape(t.ProviderID) +
		"/services/" + url.PathEscape(t.ServiceID)

	resp, err = client.New().Timeout(lookupTimeout).Get(ctx, template)
	if err != nil || resp.Status != 200 {
		return Host{}, false
	}

	return Host{Name: text(found, "providerName"), SyncUX: strings.TrimSuffix(text(found, "urlSyncUX"), "/")}, true
}

/*
text returns a text field of a host's answer, empty when absent; the
answer's field names are the standard's own.
*/
func text(found map[string]any, field string) string {
	value, _ := found[field].(string)

	return value
}

/*
Apply is one application of a template: the zone's root, the host label
under it ("" for the root itself), the template's variables, where the
host sends the customer back, and the signing key with the TXT host that
publishes its public half.
*/
type Apply struct {
	Domain      string
	Subdomain   string
	Variables   map[string]string
	RedirectURI string
	PrivateKey  string
	KeyHost     string
}

/*
ApplyURL returns the signed link to the host's consent page.
*/
func ApplyURL(host Host, t Template, a Apply) (string, error) {
	key, err := parseKey(a.PrivateKey)
	if err != nil {
		return "", err
	}

	query := url.Values{"domain": {a.Domain}}
	if a.Subdomain != "" {
		query.Set("host", a.Subdomain)
	}

	for name, value := range a.Variables {
		query.Set(name, value)
	}

	if a.RedirectURI != "" {
		query.Set("redirect_uri", a.RedirectURI)
	}

	signed := query.Encode()
	digest := sha256.Sum256([]byte(signed))

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("domainconnect: sign: %w", err)
	}

	sig := base64.StdEncoding.EncodeToString(signature)
	tail := "sig=" + url.QueryEscape(sig) + "&key=" + url.QueryEscape(a.KeyHost)

	return host.SyncUX + "/v2/domainTemplates/providers/" + url.PathEscape(t.ProviderID) + "/services/" +
		url.PathEscape(t.ServiceID) + "/apply?" + signed + "&" + tail, nil
}

/*
parseKey reads a PEM encoded RSA private key, PKCS#1 or PKCS#8.
*/
func parseKey(text string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, ErrInvalidKey
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidKey
	}

	return key, nil
}

/*
firstNonEmpty returns the first text that is set.
*/
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}
