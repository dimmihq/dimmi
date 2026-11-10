/*
|--------------------------------------------------------------------------
| Answer Sources
|--------------------------------------------------------------------------
|
| An agent's answer names the articles it rests on, so a customer on a
| chat app can read the source the way they would in the messenger. The
| channel only lists them; the answer already says what they hold.
|
*/

package contracts

/*
OutboundSource is one article an agent's answer cites.
*/
type OutboundSource struct {
	Title string
	URL   string
}
