/*
|--------------------------------------------------------------------------
| Feature Request Board
|--------------------------------------------------------------------------
|
| What the portal's board shows: the statuses requests move through,
| each request with its votes, and the board as a roadmap or a list.
|
*/

package helpcenter

/*
Status is a stage feature requests move through, with the hex color of
its icon and how many requests are in it.
*/
type Status struct {
	Name  string
	Slug  string
	Color string
	Count int
	Ideas []Idea
}

/*
Idea is a feature request: what is asked for, how many voted for it and
whether the signed-in visitor did, and how many comments it has. Path
is its own page, empty where it has none.
*/
type Idea struct {
	Slug     string
	Title    string
	Details  string
	Icon     string
	Votes    int
	Voted    bool
	Comments int
	Path     string
}

/*
BoardTab is one board of feature requests the visitor may open, such as
feature requests or bugs, by the slug its tab and form name it with.
*/
type BoardTab struct {
	Name        string
	Slug        string
	Description string
}

/*
Board is the feature requests page: the requests grouped by status as a
roadmap, or one List when Sort or Query asks for it. Boards are the tabs
to narrow it to one board, shown when there is more than one, and Active
the slug of the board shown, empty for all of them. Submitted thanks a
visitor who just posted; Pending says their post waits for approval.
*/
type Board struct {
	Boards    []BoardTab
	Active    string
	Statuses  []Status
	List      []Idea
	Sort      string
	Query     string
	Submitted bool
	Pending   bool
}
