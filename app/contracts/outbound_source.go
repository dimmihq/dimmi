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

import "github.com/dimmihq/dimmi/app/models"

/*
OutboundSource is one article an agent's answer cites.
*/
type OutboundSource struct {
	Title string
	URL   string
}

/*
SourcesOf returns the cited articles that have an address.
*/
func SourcesOf(cited []models.AnswerSource) []OutboundSource {
	out := make([]OutboundSource, 0, len(cited))
	for _, s := range cited {
		if s.URL != "" {
			out = append(out, OutboundSource{Title: s.Title, URL: s.URL})
		}
	}

	return out
}

/*
ChatAnswer is a sent answer as it stays once its controls are gone: its
words and the articles it cites.
*/
type ChatAnswer struct {
	Body    string
	Sources []OutboundSource
}
