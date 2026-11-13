/*
|--------------------------------------------------------------------------
| Widget Message Parts
|--------------------------------------------------------------------------
|
| What a message carries beside its body, for a messenger that draws it:
| the answers a workflow's question offers, and a help article it
| answered with.
|
*/

package resources

/*
WidgetChoice is one answer a question offers: the path the workflow takes
when it is tapped, and what the button reads.
*/
type WidgetChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

/*
WidgetArticleCard is a help article a workflow answered with, as a card
the messenger opens in place.
*/
type WidgetArticleCard struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	URL     string `json:"url,omitempty"`
}
