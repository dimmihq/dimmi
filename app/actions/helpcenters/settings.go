/*
|--------------------------------------------------------------------------
| Help Center Settings Changes
|--------------------------------------------------------------------------
|
| A change to a help center's settings names only the settings it
| touches. Everything it leaves out stays as it was, so a client that
| sends one setting never wipes the rest. An empty text clears a setting.
|
*/

package helpcenters

/*
HelpCenterSettingsChange is a change to a help center's settings, one
field per setting; nil leaves a setting as it is, and an empty text
clears it.
*/
type HelpCenterSettingsChange struct {
	PrimaryColor     *string
	Appearance       *string
	HomeTitle        *string
	HomeSubtitle     *string
	FooterText       *string
	ContactURL       *string
	LoginURL         *string
	EmailSignIn      *bool
	FeedbackBoard    *bool
	FeedbackApproval *bool
	Messenger        *bool
	MessengerID      *string

	Changelog              *bool
	ChangelogTitle         *string
	ChangelogAuthors       *bool
	ChangelogReactions     *bool
	ChangelogSubscriptions *bool
	ChangelogEmail         *string
	ChangelogUndoMinutes   *int
}

/*
Over returns the settings with this change written over them.
*/
func (c HelpCenterSettingsChange) Over(s HelpCenterSettings) HelpCenterSettings {
	keep(&s.PrimaryColor, c.PrimaryColor)
	keep(&s.Appearance, c.Appearance)
	keep(&s.HomeTitle, c.HomeTitle)
	keep(&s.HomeSubtitle, c.HomeSubtitle)
	keep(&s.FooterText, c.FooterText)
	keep(&s.ContactURL, c.ContactURL)
	keep(&s.LoginURL, c.LoginURL)
	keep(&s.FeedbackBoard, c.FeedbackBoard)
	keep(&s.FeedbackApproval, c.FeedbackApproval)
	keep(&s.MessengerID, c.MessengerID)
	if c.EmailSignIn != nil {
		s.EmailSignIn = c.EmailSignIn
	}
	if c.Messenger != nil {
		s.Messenger = c.Messenger
	}
	return c.overChangelog(s)
}

/*
keep writes a changed value over the current one, and leaves the
current one when the change does not name it.
*/
func keep[T any](current, changed *T) {
	if changed != nil {
		*current = *changed
	}
}
