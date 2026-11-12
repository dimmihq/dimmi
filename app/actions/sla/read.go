/*
|--------------------------------------------------------------------------
| Reading Clocks
|--------------------------------------------------------------------------
|
| The clocks of a ticket or a conversation, newest first, for those who
| may see the ticket or the conversation.
|
*/

package sla

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketClocks returns every clock of a ticket the member may read.
*/
func (s *SLA) TicketClocks(ctx context.Context, m authz.Membership, ref tickets.Ref) ([]models.SlaClock, error) {
	var out []models.SlaClock

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		t, err := tickets.Readable(ctx, q, m, ref)
		if err != nil {
			return err
		}

		out, err = q.ListSubjectClocks(ctx, models.ListSubjectClocksParams{WorkspaceID: t.WorkspaceID, TicketID: &t.ID})
		if err != nil {
			return fmt.Errorf("list ticket sla clocks: %w", err)
		}

		return nil
	})

	return out, err
}

/*
ConversationClocks returns every clock of a conversation the member
may read, its own and its customer ticket's.
*/
func (s *SLA) ConversationClocks(
	ctx context.Context, m authz.Membership, ref conversations.Ref,
) ([]models.SlaClock, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []models.SlaClock

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		params := models.ListSubjectClocksParams{WorkspaceID: c.WorkspaceID, ConversationID: &c.ID}
		t, err := q.FindConversationTicket(ctx, models.FindConversationTicketParams{
			WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
		})
		if err == nil && t.DeletedAt == nil {
			params.TicketID = &t.ID
		}

		out, err = q.ListSubjectClocks(ctx, params)
		if err != nil {
			return fmt.Errorf("list conversation sla clocks: %w", err)
		}

		return nil
	})

	return out, err
}
