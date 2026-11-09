/*
|--------------------------------------------------------------------------
| Help Center Pages
|--------------------------------------------------------------------------
|
| What the public help center's pages show, already shaped for reading:
| dates are formatted, Markdown is rendered and sanitized, and every link
| is a path under the help center's base, so the same pages work on a
| subdomain, a custom domain or under the app's /hc path. Nothing here
| reaches the database; the controllers fill these in.
|
*/

package helpcenter

import "time"

/*
Center is the portal a page belongs to: its name, the scheme and host
it is served on, the path every link starts with, its brand color as a
hex code, the paths of its logo for light and dark pages (empty to draw
its initial on the brand), the address of the product it supports, the
sections it shows beside the help center, and the visitor signed in to
it, if any. SignInURL is where a reader signs in, empty when the help
center takes no sign-in; SignedIn says a reader is signed in, and
SignOutURL ends that. Appearance is the theme the team set: light or
dark for every reader, or auto, or empty, to let each reader pick.
Messenger is the public id of the messenger its pages load from
MessengerSrc, empty to load none; MessengerAI says the AI agent answers
it first, and the messenger asks MessengerIdentity who is signed in.
HomeTitle is the question the home page asks, empty for the default,
and FooterText closes every page.
*/
type Center struct {
	Name       string
	Product    string
	Origin     string
	Base       string
	Brand      string
	LogoLight  string
	LogoDark   string
	Tagline    string
	HomeURL    string
	ContactURL string
	Sections   []Section
	Visitor    *Person
	SignInURL  string
	SignedIn   bool
	SignOutURL string
	Appearance string

	Messenger    string
	MessengerSrc string
	MessengerAI  bool

	MessengerIdentity string
	HomeTitle         string
	FooterText        string
}

/*
Section is a part of the portal, such as the changelog, with the key
pages use to mark it current, its icon and the path under the base it
lives at.
*/
type Section struct {
	Key  string
	Name string
	Icon string
	Path string
}

/*
Person is an author shown with an article or a changelog entry, by name
and the initials drawn when there is no photo.
*/
type Person struct {
	Name     string
	Initials string
	Role     string
}

/*
Crumb is a step of the trail from the help center's home to a page.
*/
type Crumb struct {
	Title string
	Path  string
}

/*
Update is a changelog entry as the help center's home lists it.
*/
type Update struct {
	Title     string
	Path      string
	Label     string
	Published time.Time
}
