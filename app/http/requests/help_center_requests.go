/*
|--------------------------------------------------------------------------
| Help Center Requests
|--------------------------------------------------------------------------
|
| The inputs of help centers. Fields left out of a change stay as they
| are.
|
*/

package requests

/*
HelpCenterRequest names a help center in the path.
*/
type HelpCenterRequest struct {
	HelpCenter string `path:"help_center"`
}

/*
HelpCenterSettingsBody is how a help center looks. Only the settings it
names change; an empty text clears one.
*/
type HelpCenterSettingsBody struct {
	PrimaryColor *string `json:"primary_color,omitempty" maxLength:"7"`
	Appearance   *string `json:"appearance,omitempty"    enum:"auto,light,dark"`
	HomeTitle    *string `json:"home_title,omitempty"    maxLength:"80"`
	HomeSubtitle *string `json:"home_subtitle,omitempty" maxLength:"200"`
	FooterText   *string `json:"footer_text,omitempty"   maxLength:"500"`
	ContactURL   *string `json:"contact_url,omitempty"   maxLength:"2048"`
	LoginURL     *string `json:"login_url,omitempty"     maxLength:"2048"`
	EmailSignIn  *bool   `json:"email_sign_in,omitempty"`

	FeedbackBoard    *bool   `json:"feedback_board,omitempty"`
	FeedbackApproval *bool   `json:"feedback_approval,omitempty"`
	Messenger        *bool   `json:"messenger,omitempty"`
	MessengerID      *string `json:"messenger_id,omitempty"     maxLength:"64"`

	Changelog              *bool   `json:"changelog,omitempty"`
	ChangelogTitle         *string `json:"changelog_title,omitempty"         maxLength:"60"`
	ChangelogAuthors       *bool   `json:"changelog_authors,omitempty"`
	ChangelogReactions     *bool   `json:"changelog_reactions,omitempty"`
	ChangelogSubscriptions *bool   `json:"changelog_subscriptions,omitempty"`
	ChangelogEmail         *string `json:"changelog_email,omitempty"         enum:"full,summary"`
	ChangelogUndoMinutes   *int    `json:"changelog_undo_minutes,omitempty"  enum:"0,5,15,60"`
}

/*
UpdateHelpCenterRequest is the input of PATCH /help-centers/{help_center}.
*/
type UpdateHelpCenterRequest struct {
	HelpCenter string `path:"help_center"`
	Body       struct {
		Name            *string                 `json:"name,omitempty"             maxLength:"80"`
		Subdomain       *string                 `json:"subdomain,omitempty"        maxLength:"40"`
		Locale          *string                 `json:"locale,omitempty"           maxLength:"16"`
		Status          *string                 `json:"status,omitempty"           enum:"active,disabled"`
		IndexingEnabled *bool                   `json:"indexing_enabled,omitempty"`
		AskAIEnabled    *bool                   `json:"ask_ai_enabled,omitempty"`
		Settings        *HelpCenterSettingsBody `json:"settings,omitempty"`
	}
}
