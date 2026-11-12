/*
|--------------------------------------------------------------------------
| Feature Request Requests
|--------------------------------------------------------------------------
|
| The inputs of listing, posting and changing a help center's feature
| requests.
|
*/

package requests

/*
IndexFeedbackPostsRequest is the input of GET
/help-centers/{help_center}/feedback-posts, narrowed by board, status,
approval and title, in the order asked for.
*/
type IndexFeedbackPostsRequest struct {
	HelpCenter string `path:"help_center"`
	Board      string `query:"board_id"   maxLength:"64"`
	Status     string `query:"status"`
	Moderation string `query:"moderation" enum:"approved,pending,spam,"`
	Query      string `query:"query"      maxLength:"200"`
	Sort       string `query:"sort"       enum:"trending,top,new,"`
}

/*
StoreFeedbackPostRequest is the input of POST
/help-centers/{help_center}/feedback-posts: a request the team posts,
for a customer when contact_id is given, on the board board_id names or
the help center's first.
*/
type StoreFeedbackPostRequest struct {
	HelpCenter string `path:"help_center"`
	Body       struct {
		Title        string `json:"title"                 maxLength:"150" minLength:"1"`
		BodyMarkdown string `json:"body_markdown,omitempty" maxLength:"65536"`
		ContactID    string `json:"contact_id,omitempty"  maxLength:"64"`
		BoardID      string `json:"board_id,omitempty"    maxLength:"64"`
	}
}

/*
FeedbackPostRequest names a request in the path.
*/
type FeedbackPostRequest struct {
	Post string `path:"post"`
}

/*
UpdateFeedbackPostRequest is the input of PATCH /feedback-posts/{post}:
new words, another board, a new status, an approval, or a merge into
another request.
*/
type UpdateFeedbackPostRequest struct {
	Post string `path:"post"`
	Body struct {
		FeedbackStatusChange
		Title            *string `json:"title,omitempty"              maxLength:"150"`
		BodyMarkdown     *string `json:"body_markdown,omitempty"      maxLength:"65536"`
		BoardID          *string `json:"board_id,omitempty"           maxLength:"64"`
		ChangelogEntryID *string `json:"changelog_entry_id,omitempty" maxLength:"64"`
		Notify           bool    `json:"notify,omitempty"`
		Message          string  `json:"message,omitempty"            maxLength:"2000"`
		Moderation       *string `json:"moderation,omitempty"         enum:"approved,spam"`
		MergeInto        *string `json:"merge_into_id,omitempty"      maxLength:"64"`
	}
}

/*
FeedbackStatusChange is the status a request moves to, by the key of
one of the workspace's statuses.
*/
type FeedbackStatusChange struct {
	Status *string `json:"status,omitempty"`
}
