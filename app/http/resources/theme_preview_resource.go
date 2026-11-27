/*
|--------------------------------------------------------------------------
| Theme Preview Resources
|--------------------------------------------------------------------------
|
| A help center page built for a theme preview: what it answered and the
| template and variables it would have rendered. HTML and times travel
| as one-key objects, {"$html": ...} and {"$time": ...}.
|
*/

package resources

/*
ThemePreview is one page of a help center, built for a preview.
*/
type ThemePreview struct {
	Object    string         `json:"object"`
	Status    int            `json:"status"`
	Location  string         `json:"location,omitempty"`
	Template  string         `json:"template,omitempty"`
	Variables map[string]any `json:"variables,omitempty"`
}
