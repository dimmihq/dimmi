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

package whatsapp

import "github.com/dimmihq/dimmi/app/drivers/channel/meta"

/*
SignatureHeader is the header Meta signs deliveries in.
*/
const SignatureHeader = meta.SignatureHeader

/*
Sign returns the signature header value of the body under the secret.
*/
func Sign(secret string, body []byte) string {
	return meta.Sign(secret, body)
}

/*
Verify reports whether the header is the secret's signature of the
body. An empty secret verifies nothing.
*/
func Verify(secret, header string, body []byte) bool {
	return meta.Verify(secret, header, body)
}
