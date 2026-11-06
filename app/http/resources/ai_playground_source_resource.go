/*
|--------------------------------------------------------------------------
| Agent Playground Sources
|--------------------------------------------------------------------------
|
| The passages a playground question found and the ones its answer
| cites, numbered the same way so the two can be read side by side.
|
*/

package resources

/*
AiPlaygroundSource is one retrieved passage.
*/
type AiPlaygroundSource struct {
	N          int     `json:"n"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity"`
	Used       bool    `json:"used"`
	Text       string  `json:"text"`
}

/*
AiPlaygroundCitation is one cited source.
*/
type AiPlaygroundCitation struct {
	N     int    `json:"n"`
	Title string `json:"title"`
	URL   string `json:"url"`
}
