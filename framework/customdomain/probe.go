/*
|--------------------------------------------------------------------------
| Routing Probe
|--------------------------------------------------------------------------
|
| DNS records can look wrong while traffic still arrives, for instance
| behind a proxy such as Cloudflare that answers with its own addresses.
| The probe settles it: it requests a path on the customer's hostname and
| expects an answer only this application can give, a keyed signature of
| a token it chose. Any other server, even one that echoes requests back,
| cannot produce it.
|
*/

package customdomain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
maxProbeBody caps how much of a probe answer is read; a real answer is
64 hex characters.
*/
const maxProbeBody = 1024

/*
Probe proves that requests for a hostname reach this application.

Path is where the probe handler is mounted, such as
"/.well-known/domain-check"; tokens are appended to it. Key signs the
answers and must be the same on every server.
*/
type Probe struct {
	Path string
	Key  []byte
}

/*
Handler answers a probe for any token under Path with the token's
signature, as plain text that is never cached.
*/
func (p Probe) Handler() http.Handler {
	prefix := strings.TrimSuffix(p.Path, "/") + "/"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.URL.Path, prefix)
		if !ok || token == "" || strings.Contains(token, "/") {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write([]byte(p.sign(token)))
	})
}

/*
Run requests the token on the hostname over plain HTTP and reports
whether the answer is this application's signature. A network error or
any other answer is a failed probe.
*/
func (p Probe) Run(ctx context.Context, client *httpclient.Client, host, token string) bool {
	url := "http://" + host + strings.TrimSuffix(p.Path, "/") + "/" + token

	resp, err := client.New().MaxBody(maxProbeBody).Get(ctx, url)
	if err != nil || resp.Status != http.StatusOK {
		return false
	}

	return hmac.Equal([]byte(strings.TrimSpace(string(resp.Body))), []byte(p.sign(token)))
}

func (p Probe) sign(token string) string {
	mac := hmac.New(sha256.New, p.Key)
	mac.Write([]byte(token))

	return hex.EncodeToString(mac.Sum(nil))
}
