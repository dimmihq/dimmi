/*
|--------------------------------------------------------------------------
| Feature Request Pages
|--------------------------------------------------------------------------
|
| What a request's own page and the form to post one show: the request
| with its comments, and the form filled in again when needed.
|
*/

package helpcenter

/*
Comment is a reply under a request, with the replies to it. FromTeam
marks the team's own.
*/
type Comment struct {
	Author   string
	FromTeam bool
	Body     string
	At       string
	Replies  []Comment
}

/*
IdeaView is one request's page: the request, where it stands, its
details as HTML, who posted it and when, its comments, the update that
shipped it, and whether the visitor may comment or must sign in first.
*/
type IdeaView struct {
	Idea      Idea
	Status    Status
	Details   string
	Author    string
	Posted    string
	Comments  []Comment
	Update    string
	Pending   bool
	CanAct    bool
	SignIn    string
	Commented bool
}

/*
IdeaForm is the form to post a request, filled in again when similar
requests are shown first or something was wrong.
*/
type IdeaForm struct {
	Title   string
	Details string
	Similar []Idea
	Error   string
}
