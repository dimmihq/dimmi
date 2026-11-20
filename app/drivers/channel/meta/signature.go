/*
|--------------------------------------------------------------------------
| Delivery Signatures
|--------------------------------------------------------------------------
|
| Meta signs every webhook delivery with the app's secret: an HMAC-SHA256
| of the raw body, sent as X-Hub-Signature-256 with a sha256= prefix.
| Nothing in a delivery is read before the signature holds, and the
| comparison takes the same time whatever it finds.
|
*/

package meta

import (
	"encoding/hex"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
SignatureHeader is the header Meta signs deliveries in.
*/
const SignatureHeader = "X-Hub-Signature-256"

/*
Sign returns the signature header value of the body under the secret.
*/
func Sign(secret string, body []byte) string {
	return "sha256=" + hex.EncodeToString(hashing.HMAC([]byte(secret), body))
}

/*
Verify reports whether the header is the secret's signature of the
body. An empty secret verifies nothing.
*/
func Verify(secret, header string, body []byte) bool {
	if secret == "" || !strings.HasPrefix(header, "sha256=") {
		return false
	}

	return hashing.Equal([]byte(Sign(secret, body)), []byte(header))
}
