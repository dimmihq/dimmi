/*
|--------------------------------------------------------------------------
| Signing Keys
|--------------------------------------------------------------------------
|
| One relay may send for many domains, each signed with its own key, so a
| message can carry the key it is signed with. A driver whose provider
| signs mail itself ignores it.
|
*/

package mail

/*
SigningKey is a DKIM key for one domain: the domain, the selector its
public half is published under, and the PEM encoded private half.
*/
type SigningKey struct {
	Domain     string
	Selector   string
	PrivateKey string
}
