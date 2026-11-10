/*
|--------------------------------------------------------------------------
| Who A Broadcast Can Reach
|--------------------------------------------------------------------------
|
| A page of matched contacts is checked in a handful of queries, never
| one per person. Service notices, such as an outage or a security
| change, reach people who left the marketing lists, but never an
| address that bounced or complained: that protects the person and the
| sending address alike.
|
*/

package broadcasts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
facts is what deciding a page needs: the suppressed addresses, who used
the Messenger, the topic choices, who left all broadcasts, when each
last got one, the topic's default, the workspace's zone, the cap and
the time the page is decided at.
*/
type facts struct {
	suppressed map[string]bool
	messenger  map[ids.UUID]bool
	choices    map[ids.UUID]bool
	optedOut   map[ids.UUID]bool
	lastSent   map[ids.UUID]time.Time
	optIn      bool
	zone       *time.Location
	capSince   time.Time
	now        time.Time
}

/*
factsOf loads the facts of one page.
*/
func (b *Broadcasts) factsOf(ctx context.Context, q *models.Queries, row models.Broadcast, page []models.Contact,
) (facts, error) {
	ws := row.WorkspaceID
	f := facts{
		suppressed: map[string]bool{}, messenger: map[ids.UUID]bool{}, choices: map[ids.UUID]bool{},
		optedOut: map[ids.UUID]bool{}, lastSent: map[ids.UUID]time.Time{},
		capSince: b.Clock.Now().Add(-time.Duration(b.Config.MinGapHours) * time.Hour), now: b.Clock.Now(),
	}

	contacts, emails := idsAndEmails(page)

	steps := []func() error{
		func() error { return f.loadZone(ctx, q, ws) },
		func() error { return f.loadSuppressed(ctx, q, row, emails) },
		func() error { return f.loadMessenger(ctx, q, row, contacts) },
		func() error { return f.loadConsent(ctx, q, row, contacts) },
		func() error { return f.loadLastSent(ctx, q, row, contacts) },
	}

	for _, step := range steps {
		if err := step(); err != nil {
			return f, err
		}
	}

	return f, nil
}

/*
leaveOut returns why a contact is not reached, or "" when they are.
*/
func (f *facts) leaveOut(row models.Broadcast, c models.Contact, address string, seen map[string]bool) string {
	if c.BlockedAt != nil {
		return ReasonBlocked
	}

	if reason := f.channelReason(row, c, address, seen); reason != "" {
		return reason
	}

	if row.Service {
		return ""
	}

	switch {
	case f.optedOut[c.ID]:
		return ReasonOptedOut
	case row.TopicID != nil && !f.subscribed(c.ID):
		return ReasonTopic
	case !row.SkipFrequencyCap && f.lastSent[c.ID].After(f.capSince):
		return ReasonFrequencyCap
	}

	return ""
}

/*
channelReason returns why the channel cannot reach the contact.
*/
func (f *facts) channelReason(row models.Broadcast, c models.Contact, address string, seen map[string]bool) string {
	if row.Channel != ChannelEmail {
		if !f.messenger[c.ID] {
			return ReasonNoMessenger
		}

		return ""
	}

	switch {
	case address == "" || !strings.Contains(address, "@"):
		return ReasonNoEmail
	case f.suppressed[address]:
		return ReasonSuppressed
	case seen[address]:
		return ReasonDuplicate
	}

	seen[address] = true

	return ""
}

/*
subscribed reports whether the contact gets the broadcast's topic: their
own choice, else the topic's default.
*/
func (f *facts) subscribed(contact ids.UUID) bool {
	if choice, ok := f.choices[contact]; ok {
		return choice
	}

	return !f.optIn
}

/*
loadZone reads the workspace's time zone.
*/
func (f *facts) loadZone(ctx context.Context, q *models.Queries, ws ids.UUID) error {
	workspace, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return fmt.Errorf("find workspace: %w", err)
	}

	f.zone = time.UTC
	if loc, err := time.LoadLocation(workspace.Timezone); err == nil {
		f.zone = loc
	}

	return nil
}

/*
loadSuppressed reads which addresses must not be mailed.
*/
func (f *facts) loadSuppressed(ctx context.Context, q *models.Queries, row models.Broadcast, emails []string) error {
	if row.Channel != ChannelEmail || len(emails) == 0 {
		return nil
	}

	rows, err := q.SuppressedEmailsAmong(ctx, models.SuppressedEmailsAmongParams{
		WorkspaceID: row.WorkspaceID, Emails: emails, Now: f.now,
	})
	for _, e := range rows {
		f.suppressed[e] = true
	}

	return wrap("read suppressions", err)
}

/*
loadMessenger reads who has used the Messenger.
*/
func (f *facts) loadMessenger(ctx context.Context, q *models.Queries, row models.Broadcast, contacts []ids.UUID) error {
	if row.Channel != ChannelChat && row.Channel != ChannelPost {
		return nil
	}

	rows, err := q.MessengerContactsAmong(ctx, models.MessengerContactsAmongParams{
		WorkspaceID: row.WorkspaceID, ContactIds: contacts,
	})
	for _, id := range rows {
		f.messenger[id] = true
	}

	return wrap("read messenger contacts", err)
}

/*
loadConsent reads who left all broadcasts and, for a broadcast on a
topic, what each chose about it.
*/
func (f *facts) loadConsent(ctx context.Context, q *models.Queries, row models.Broadcast, contacts []ids.UUID) error {
	ws := row.WorkspaceID

	out, err := q.OptedOutAmong(ctx, models.OptedOutAmongParams{WorkspaceID: ws, ContactIds: contacts})
	if err != nil {
		return fmt.Errorf("read opt-outs: %w", err)
	}

	for _, id := range out {
		f.optedOut[id] = true
	}

	if row.TopicID == nil {
		return nil
	}

	topic, err := q.FindBroadcastTopic(ctx, models.FindBroadcastTopicParams{WorkspaceID: ws, ID: *row.TopicID})
	if err != nil {
		return fmt.Errorf("find topic: %w", err)
	}

	f.optIn = topic.Consent == "opt_in"

	choices, err := q.TopicChoicesAmong(ctx, models.TopicChoicesAmongParams{
		WorkspaceID: ws, TopicID: *row.TopicID, ContactIds: contacts,
	})
	for _, c := range choices {
		f.choices[c.ContactID] = c.Subscribed
	}

	return wrap("read topic choices", err)
}

/*
loadLastSent reads when each contact last got a capped broadcast.
*/
func (f *facts) loadLastSent(ctx context.Context, q *models.Queries, row models.Broadcast, contacts []ids.UUID) error {
	if row.Service || row.SkipFrequencyCap {
		return nil
	}

	rows, err := q.LastBroadcastSentAmong(ctx, models.LastBroadcastSentAmongParams{
		WorkspaceID: row.WorkspaceID, ContactIds: contacts, BroadcastID: row.ID,
	})
	for _, r := range rows {
		if r.ContactID != nil {
			f.lastSent[*r.ContactID] = r.LastSent
		}
	}

	return wrap("read last broadcasts", err)
}

/*
idsAndEmails lists a page's contact ids and lowercased addresses.
*/
func idsAndEmails(page []models.Contact) ([]ids.UUID, []string) {
	contacts := make([]ids.UUID, 0, len(page))
	emails := make([]string, 0, len(page))

	for _, c := range page {
		contacts = append(contacts, c.ID)
		if c.Email != nil && *c.Email != "" {
			emails = append(emails, strings.ToLower(strings.TrimSpace(*c.Email)))
		}
	}

	return contacts, emails
}
