/*
|--------------------------------------------------------------------------
| Reports
|--------------------------------------------------------------------------
|
| A report says what a broadcast did: how many it reached on its
| channel, what they did with it, why the rest were left out, which
| links drew clicks and, for B2B, how each account responded. The
| recipient list is searchable by what happened, so a team can follow up
| with exactly the people who clicked but never replied.
|
*/

package broadcasts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Report is a broadcast's results: its counts, why recipients were left
out, its links by clicks, each account's results and why recipients
still queued are waiting.
*/
type Report struct {
	Broadcast models.Broadcast
	Counts    models.TallyBroadcastRecipientsRow
	Reasons   []models.CountBroadcastSkipReasonsRow
	Links     []models.BroadcastLink
	Companies []models.BroadcastCompanyReportRow
	Waiting   []models.CountPacedBroadcastRecipientsRow
}

/*
Report returns the broadcast's results.
*/
func (b *Broadcasts) Report(ctx context.Context, m authz.Membership, id ids.UUID) (Report, error) {
	if !m.Can("broadcasts:read") {
		return Report{}, exceptions.ErrForbidden
	}

	var out Report

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		ws := m.Workspace.ID

		var err error
		if out.Broadcast, err = find(ctx, q, ws, id); err != nil {
			return err
		}

		if out.Counts, err = q.TallyBroadcastRecipients(ctx, models.TallyBroadcastRecipientsParams{
			WorkspaceID: ws, BroadcastID: id,
		}); err != nil {
			return wrap("tally", err)
		}

		if out.Reasons, err = q.CountBroadcastSkipReasons(ctx, models.CountBroadcastSkipReasonsParams{
			WorkspaceID: ws, BroadcastID: id,
		}); err != nil {
			return wrap("count reasons", err)
		}

		if out.Links, err = q.ListBroadcastLinks(ctx, models.ListBroadcastLinksParams{
			WorkspaceID: ws, BroadcastID: id,
		}); err != nil {
			return wrap("list links", err)
		}

		if out.Companies, err = q.BroadcastCompanyReport(ctx, models.BroadcastCompanyReportParams{
			WorkspaceID: ws, BroadcastID: id,
		}); err != nil {
			return wrap("report companies", err)
		}

		out.Waiting, err = q.CountPacedBroadcastRecipients(ctx, models.CountPacedBroadcastRecipientsParams{
			WorkspaceID: ws, BroadcastID: id,
		})

		return wrap("count waiting", err)
	})

	return out, err
}

/*
RecipientsInput narrows the recipient list: by what happened, by
account, a page at a time.
*/
type RecipientsInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Status     *string
	CompanyID  *ids.UUID
	Limit      int
	Offset     int
}

/*
Recipients returns a page of a broadcast's recipients.
*/
func (b *Broadcasts) Recipients(
	ctx context.Context, in RecipientsInput) ([]models.ListBroadcastRecipientPageRow, error,
) {
	if !in.Membership.Can("broadcasts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.ListBroadcastRecipientPageRow

	err := b.inWorkspace(ctx, in.Membership.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		if _, err := find(ctx, q, in.Membership.Workspace.ID, in.ID); err != nil {
			return err
		}

		var err error

		out, err = q.ListBroadcastRecipientPage(ctx, models.ListBroadcastRecipientPageParams{
			WorkspaceID: in.Membership.Workspace.ID, BroadcastID: in.ID, Status: in.Status, CompanyID: in.CompanyID,
			RowLimit: int64(max(min(in.Limit, 100), 1)), RowOffset: int64(max(in.Offset, 0)),
		})

		return wrap("list recipients", err)
	})

	return out, err
}
