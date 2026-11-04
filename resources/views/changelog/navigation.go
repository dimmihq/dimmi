/*
|--------------------------------------------------------------------------
| Changelog Navigation
|--------------------------------------------------------------------------
|
| How a visitor moves through the changelog besides scrolling: the strip
| of the year's weeks with a mark for every release, and an entry's
| neighbors before and after it.
|
*/

package changelog

/*
Week is a week of the release strip: the month it opens, when it is the
first week of one, and the entry published in it, if any.
*/
type Week struct {
	Month string
	Title string
	Path  string
}

/*
Page is a single entry with the entries before and after it.
*/
type Page struct {
	Entry    Entry
	Previous *Entry
	Next     *Entry
}
