/*
|--------------------------------------------------------------------------
| Help Center Languages
|--------------------------------------------------------------------------
|
| The languages a page is offered in: the menu that switches between
| them, and the alternates search engines read to find each one.
|
*/

package helpcenter

/*
LanguageLink is a language in the language menu: its tag, its name in
itself, where the page is in it and whether it is the page's language.
*/
type LanguageLink struct {
	Code    string
	Name    string
	URL     string
	Current bool
}

/*
Alternate is a page's address in one language, or x-default for the
one a reader of none of them gets.
*/
type Alternate struct {
	Lang string
	URL  string
}
