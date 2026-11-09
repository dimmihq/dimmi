/*
|--------------------------------------------------------------------------
| Mailing an Entry
|--------------------------------------------------------------------------
|
| An entry the team chose to send goes to every active subscriber once.
| The mailing claims the entry first, so a retried or doubled job finds
| it already sent and stops. Each subscriber gets their own mail, queued
| a batch at a time, with their own link to unsubscribe.
|
*/

package changelog

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/mail/changelog"
)

/*
mailingBatch is how many subscribers a mailing queues at a time.
*/
const mailingBatch = 500

/*
MailEntry sends a published entry to its changelog's active subscribers,
once.
*/
func (s *Subscribers) MailEntry(ctx context.Context, args MailEntryArgs) error {
	var (
		entry  models.ChangelogEntry
		center models.HelpCenter
	)
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		entry, err = q.ClaimChangelogMailing(ctx, models.ClaimChangelogMailingParams{
			Now: new(s.Clock.Now()), WorkspaceID: args.WorkspaceID, ID: args.EntryID,
		})
		if err != nil {
			return err
		}
		center, err = q.FindHelpCenter(ctx, models.FindHelpCenterParams{
			WorkspaceID: args.WorkspaceID, ID: entry.HelpCenterID,
		})
		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim changelog mailing: %w", err)
	}
	mail := s.entryMail(center, entry)
	after := ids.UUID{}
	for {
		next, err := s.mailBatch(ctx, center, mail, after)
		if err != nil || next == (ids.UUID{}) {
			return err
		}
		after = next
	}
}

/*
entryMail is the entry as every subscriber's mail shows it.
*/
func (s *Subscribers) entryMail(h models.HelpCenter, e models.ChangelogEntry) changelog.Entry {
	root := s.Addresses.Root(h)
	out := changelog.Entry{
		HelpCenter: h.Name, Brand: brandOf(h), Title: deref(e.PublishedTitle), Summary: deref(e.PublishedSummary),
		BodyHTML: absolute(deref(e.PublishedBodyHtml), root), URL: root + "/changelog/" + e.Slug,
	}
	if cover := deref(e.PublishedCoverUrl); cover != "" {
		out.CoverURL = root + cover
	}
	return out
}

/*
mailBatch queues the next batch of subscribers' mail and returns the
last one it reached, or none when the batch was the last.
*/
func (s *Subscribers) mailBatch(ctx context.Context, h models.HelpCenter, e changelog.Entry, after ids.UUID,
) (ids.UUID, error) {
	var last ids.UUID
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		subs, err := models.New(tx).ListActiveChangelogSubscribers(ctx, models.ListActiveChangelogSubscribersParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, After: after, RowLimit: mailingBatch,
		})
		if err != nil {
			return fmt.Errorf("list changelog subscribers to mail: %w", err)
		}
		for _, sub := range subs {
			one := e
			one.Unsubscribe = s.Link(h, sub, PurposeUnsubscribe)
			m := appmail.ChangelogEntryMail{Entry: one, OneClick: one.Unsubscribe}
			if err := s.Mail.To(fmail.Address{Email: sub.Email}).QueueTx(ctx, tx, m); err != nil {
				return fmt.Errorf("queue changelog mail: %w", err)
			}
		}
		if len(subs) == mailingBatch {
			last = subs[len(subs)-1].ID
		}
		return nil
	})
	return last, err
}
