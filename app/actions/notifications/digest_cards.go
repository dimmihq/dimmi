/*
|--------------------------------------------------------------------------
| Digest Cards
|--------------------------------------------------------------------------
|
| Each digest is one short card a team can read in passing: numbers in
| words, the people by name, and a link back into dimmi for anything
| worth a click. Subjects and names come from customers and teammates,
| so they are escaped like any other words.
|
*/

package notifications

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
)

/*
cards renders the destination's digests, in the order it lists them.
*/
func (d *Digests) cards(ctx context.Context, in digest) []slackdriver.Card {
	var out []slackdriver.Card

	for _, kind := range in.row.Digests {
		switch kind {
		case DigestStandup:
			out = append(out, standupCard(in))
		case DigestSummary:
			out = append(out, d.summaryCard(in))
		case DigestThemes:
			if in.monday {
				out = append(out, d.themesCard(ctx, in))
			}
		}
	}

	return out
}

/*
standupCard lists each teammate's open queue.
*/
func standupCard(in digest) slackdriver.Card {
	title := "Daily standup for " + in.workspace.Name
	lines := []string{"*" + slackdriver.Escape(title) + "*"}

	var queue []string

	for _, r := range in.standup {
		if r.OpenCount == 0 {
			continue
		}

		queue = append(queue, fmt.Sprintf("• %s: %s open, %d waiting on a reply", slackdriver.Escape(r.MemberName),
			strconv.FormatInt(r.OpenCount, 10), r.WaitingCount))
	}

	if in.nobody > 0 {
		queue = append(queue, fmt.Sprintf("• Nobody yet: %d open", in.nobody))
	}

	if len(queue) == 0 {
		queue = []string{"Nobody has an open conversation. Inbox zero."}
	}

	return slackdriver.Card{Text: title, Lines: append(lines, strings.Join(queue, "\n"))}
}

/*
summaryCard sums up the last day.
*/
func (d *Digests) summaryCard(in digest) slackdriver.Card {
	s := in.summary
	title := "Yesterday in " + in.workspace.Name
	lines := []string{
		"*" + slackdriver.Escape(title) + "*",
		fmt.Sprintf("%d opened · %d closed · median first reply %s", s.Opened, s.Closed,
			duration(s.MedianFirstReplySeconds)),
		fmt.Sprintf("The agent answered %d on its own and handed %d to the team.", s.AgentAnswered, s.HandedOff),
	}

	if in.oldest != nil {
		subject := "#" + strconv.FormatInt(in.oldest.Number, 10)
		if in.oldest.Subject != nil && *in.oldest.Subject != "" {
			subject += " " + *in.oldest.Subject
		}

		link := d.Links.Conversation(in.workspace.Slug, in.oldest.ID)
		lines = append(lines, "Waiting longest without a reply: "+slackdriver.LinkText(link, subject))
	}

	return slackdriver.Card{Text: title, Lines: lines}
}

/*
themesCard names what customers wrote about most last week.
*/
func (d *Digests) themesCard(ctx context.Context, in digest) slackdriver.Card {
	title := "Last week's themes in " + in.workspace.Name
	themes := d.themes(ctx, in)

	if len(themes) == 0 {
		return slackdriver.Card{Text: title, Lines: []string{
			"*" + slackdriver.Escape(title) + "*", "Too few conversations last week to find themes.",
		}}
	}

	list := make([]string, 0, len(themes))
	for i, t := range themes {
		list = append(list, fmt.Sprintf("%d. %s (%d)", i+1, slackdriver.Escape(t.Name), t.Count))
	}

	return slackdriver.Card{Text: title, Lines: []string{
		"*" + slackdriver.Escape(title) + "*", strings.Join(list, "\n"),
	}}
}

/*
themes asks the theme finder, falling back to the week's tags when
there is none or it fails.
*/
func (d *Digests) themes(ctx context.Context, in digest) []Theme {
	if d.Themes != nil && len(in.subjects) > 0 {
		found, err := d.Themes.Themes(ctx, in.row.WorkspaceID, in.subjects)
		if err == nil && len(found) > 0 {
			return found[:min(len(found), themesShown)]
		}
	}

	out := make([]Theme, 0, len(in.tags))
	for _, t := range in.tags {
		out = append(out, Theme{Name: t.Name, Count: int(t.Uses)})
	}

	return out
}

/*
duration writes seconds as minutes or hours, a dash for none.
*/
func duration(seconds float64) string {
	if seconds <= 0 {
		return "–"
	}

	d := time.Duration(seconds) * time.Second
	if d < time.Hour {
		return strconv.Itoa(max(1, int(d.Minutes()))) + "m"
	}

	return strconv.FormatFloat(d.Hours(), 'f', 1, 64) + "h"
}
