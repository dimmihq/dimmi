/*
|--------------------------------------------------------------------------
| Statuses
|--------------------------------------------------------------------------
|
| Where a feature request stands, in the order the roadmap shows them:
| under consideration until the team decides, then planned, in progress
| and shipped, or closed when the team will not build it. Closed
| requests leave the public roadmap but keep their page.
|
*/

package feedback

/*
Status is one stage a request moves through: its stored key, the name
and slug readers see, and its color.
*/
type Status struct {
	Key   string
	Name  string
	Slug  string
	Color string
}

/*
Statuses returns every status in the built-in order a workspace starts
with.
*/
func Statuses() []Status {
	return []Status{
		{Key: "in_progress", Name: "In progress", Slug: "in-progress", Color: "#f59e0b"},
		{Key: "planned", Name: "Planned", Slug: "planned", Color: "#3b82f6"},
		{Key: "under_review", Name: "Under consideration", Slug: "under-consideration", Color: "#a1a1aa"},
		{Key: "shipped", Name: "Shipped", Slug: "shipped", Color: "#16a34a"},
		{Key: "closed", Name: "Closed", Slug: "closed", Color: "#71717a"},
	}
}

/*
StatusOf returns the status with the key, and whether there is one.
*/
func StatusOf(key string) (Status, bool) {
	for _, s := range Statuses() {
		if s.Key == key {
			return s, true
		}
	}
	return Status{}, false
}
