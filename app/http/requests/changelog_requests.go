/*
|--------------------------------------------------------------------------
| Changelog Entry Requests
|--------------------------------------------------------------------------
|
| The inputs of listing, writing and saving a changelog's entries.
| Labels and authors are lists of ids that replace what the entry had;
| an empty list of authors goes back to naming the editors.
|
*/

package requests

/*
IndexChangelogEntriesRequest is the input of GET
/help-centers/{help_center}/changelog-entries: a page of entries,
narrowed by state, label and title.
*/
type IndexChangelogEntriesRequest struct {
	HelpCenter string `path:"help_center"`
	Status     string `query:"status"   enum:"draft,scheduled,published,"`
	Label      string `query:"label_id" maxLength:"64"`
	Query      string `query:"query"    maxLength:"200"`
}

/*
ChangelogEntryBody is an entry's fields.
*/
type ChangelogEntryBody struct {
	Title        *string   `json:"title,omitempty"         maxLength:"150"`
	Slug         *string   `json:"slug,omitempty"          maxLength:"120"`
	Summary      *string   `json:"summary,omitempty"       maxLength:"300"`
	BodyMarkdown *string   `json:"body_markdown,omitempty" maxLength:"262144"`
	CoverURL     *string   `json:"cover_url,omitempty"     maxLength:"500"`
	LabelIDs     *[]string `json:"label_ids,omitempty"     maxItems:"20"`
	AuthorIDs    *[]string `json:"author_ids,omitempty"    maxItems:"5"`
}

/*
StoreChangelogEntryRequest is the input of POST
/help-centers/{help_center}/changelog-entries.
*/
type StoreChangelogEntryRequest struct {
	HelpCenter string `path:"help_center"`
	Body       ChangelogEntryBody
}

/*
ChangelogEntryRequest names an entry in the path.
*/
type ChangelogEntryRequest struct {
	Entry string `path:"entry"`
}

/*
UpdateChangelogEntryRequest is the input of PATCH
/changelog-entries/{entry}: a save of the working copy from the lock
version the editor started at.
*/
type UpdateChangelogEntryRequest struct {
	Entry string `path:"entry"`
	Body  struct {
		ChangelogEntryBody
		LockVersion int32 `json:"lock_version,omitempty"`
	}
}
