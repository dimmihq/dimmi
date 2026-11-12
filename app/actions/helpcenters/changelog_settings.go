/*
|--------------------------------------------------------------------------
| Changelog Settings
|--------------------------------------------------------------------------
|
| How a help center's changelog shows and mails: whether it is on at
| all, its title, whether entries name their authors, whether readers
| react and subscribe, whether the mail holds the whole entry or its
| summary with a link, and how long the team has to take an entry back
| before it is mailed. Every switch is on until the team turns it off,
| so a help center that never opened these settings keeps the changelog
| it always had.
|
*/

package helpcenters

import (
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ChangelogOptions is a help center's changelog settings with every
default filled in. Title is what its pages are headed; Summary mails
the summary instead of the whole entry; Undo is how long after
publishing an entry is mailed.
*/
type ChangelogOptions struct {
	On            bool
	Title         string
	Authors       bool
	Reactions     bool
	Subscriptions bool
	Summary       bool
	Undo          time.Duration
}

/*
UndoChoices returns the minutes the team may wait before mailing.
*/
func UndoChoices() []int { return []int{0, 5, 15, 60} }

/*
ChangelogOf reads a help center's changelog settings.
*/
func ChangelogOf(h models.HelpCenter) ChangelogOptions {
	s := SettingsOf(h)
	out := ChangelogOptions{
		On: onUnlessOff(s.Changelog), Title: s.ChangelogTitle, Authors: onUnlessOff(s.ChangelogAuthors),
		Reactions: onUnlessOff(s.ChangelogReactions), Subscriptions: onUnlessOff(s.ChangelogSubscriptions),
		Summary: s.ChangelogEmail == "summary",
	}
	if out.Title == "" {
		out.Title = "Changelog"
	}
	if s.ChangelogUndoMinutes != nil {
		out.Undo = time.Duration(*s.ChangelogUndoMinutes) * time.Minute
	}
	return out
}

func onUnlessOff(b *bool) bool {
	return b == nil || *b
}

/*
overChangelog writes the change's changelog settings over the current
ones.
*/
func (c HelpCenterSettingsChange) overChangelog(s HelpCenterSettings) HelpCenterSettings {
	keep(&s.ChangelogTitle, c.ChangelogTitle)
	keep(&s.ChangelogEmail, c.ChangelogEmail)
	for _, pair := range []struct {
		to   **bool
		from *bool
	}{
		{&s.Changelog, c.Changelog},
		{&s.ChangelogAuthors, c.ChangelogAuthors},
		{&s.ChangelogReactions, c.ChangelogReactions},
		{&s.ChangelogSubscriptions, c.ChangelogSubscriptions},
	} {
		if pair.from != nil {
			*pair.to = pair.from
		}
	}
	if c.ChangelogUndoMinutes != nil {
		s.ChangelogUndoMinutes = c.ChangelogUndoMinutes
	}
	return s
}

/*
checkChangelog refuses changelog settings out of bounds.
*/
func checkChangelog(s HelpCenterSettings) error {
	if s.ChangelogEmail != "" && s.ChangelogEmail != "full" && s.ChangelogEmail != "summary" {
		return fault.New(fault.Invalid, "changelog_email_invalid", "The email holds the full entry or a summary.").
			OnField("settings.changelog_email")
	}
	if s.ChangelogUndoMinutes != nil && !slices.Contains(UndoChoices(), *s.ChangelogUndoMinutes) {
		return fault.New(fault.Invalid, "changelog_undo_minutes_invalid", "Wait 0, 5, 15 or 60 minutes.").
			OnField("settings.changelog_undo_minutes")
	}
	return nil
}
