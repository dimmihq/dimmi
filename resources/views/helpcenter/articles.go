/*
|--------------------------------------------------------------------------
| Help Center Articles
|--------------------------------------------------------------------------
|
| The help center's own pages read these: collections and their articles,
| an article rendered for reading with its contents and neighbors, and
| what a search found.
|
*/

package helpcenter

import "time"

/*
Collection is a group of articles with its sub-collections. Icon is a
word naming one of the help center's icons. Count is the number of
articles in it and its sub-collections.
*/
type Collection struct {
	Slug        string
	Title       string
	Description string
	Icon        string
	Count       int
	Authors     []Person
	Articles    []Summary
	Children    []Collection
}

/*
Summary is an article as a list shows it.
*/
type Summary struct {
	Slug        string
	Title       string
	Description string
	Collection  string
}

/*
Heading is an entry of an article's table of contents, with the
headings nested under it.
*/
type Heading struct {
	Title    string
	ID       string
	Children []Heading
}

/*
Article is an article's page: its trail, rendered body and contents, who
wrote it, when it last changed and how long ago that is in Age, and its
neighbors in the collection. Untranslated names, in itself, the language
the page is read in when the article is not translated into it and is
shown in OriginalLang, its default language, instead; empty for an
article read in its own language.
*/
type Article struct {
	Summary
	Trail    []Crumb
	Body     string
	Contents []Heading
	Author   Person
	Updated  time.Time
	Age      string
	Minutes  int
	Previous *Summary
	Next     *Summary
	Related  []Summary

	Untranslated string
	OriginalLang string
}

/*
Result is a search hit: an article or a changelog entry, with Excerpt
holding the matching text.
*/
type Result struct {
	Kind    string
	Title   string
	Path    string
	Excerpt string
	Trail   string
}
