/*
|--------------------------------------------------------------------------
| Finding Tickets
|--------------------------------------------------------------------------
|
| A ticket is named by its id or its reference, T-482. A member sees a
| ticket on the same terms as a conversation: by its inbox, teammate
| and team, so access settings that hide conversations hide the tickets
| beside them. A ticket the member may not see is not found.
|
*/

package tickets

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Ref names a ticket by id or by number.
*/
type Ref struct {
	ID     ids.UUID
	Number int64
}

/*
ParseNumber reads a reference such as T-482 or 482, and reports false
for anything else.
*/
func ParseNumber(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "T-"), "t-")

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, false
	}

	return n, true
}

/*
Reference writes a ticket's number as the team reads it.
*/
func Reference(number int64) string {
	return "T-" + strconv.FormatInt(number, 10)
}

/*
notFound turns a missing row into ErrTicketNotFound.
*/
func notFound(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return ErrTicketNotFound
	}

	return fmt.Errorf("find ticket: %w", err)
}

/*
findRow reads the ticket the reference names, trashed or not.
*/
func findRow(ctx context.Context, q *models.Queries, ws ids.UUID, ref Ref) (models.Ticket, error) {
	var (
		t   models.Ticket
		err error
	)

	if ref.ID == ids.Nil() {
		t, err = q.FindTicketByNumber(ctx, models.FindTicketByNumberParams{WorkspaceID: ws, Number: ref.Number})
	} else {
		t, err = q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: ws, ID: ref.ID})
	}

	if err != nil {
		return t, notFound(err)
	}

	return t, nil
}

/*
sees reports whether the member may see the ticket.
*/
func sees(m authz.Membership, t models.Ticket) bool {
	return m.ConversationScope().Sees(t.InboxID, t.AssigneeMemberID, t.AssigneeTeamID)
}

/*
find returns the live ticket the member may see, or ErrTicketNotFound.
*/
func find(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) (models.Ticket, error) {
	t, err := findAny(ctx, q, m, ref)
	if err == nil && t.DeletedAt != nil {
		return t, ErrTicketNotFound
	}

	return t, err
}

/*
findAny returns the ticket the member may see, in the trash too, or
ErrTicketNotFound.
*/
func findAny(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) (models.Ticket, error) {
	t, err := findRow(ctx, q, m.Workspace.ID, ref)
	if err != nil {
		return t, err
	}

	if !sees(m, t) {
		return t, ErrTicketNotFound
	}

	return t, nil
}

/*
canRead refuses a member who may not read tickets.
*/
func canRead(m authz.Membership) error {
	if !m.Can("tickets:read") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
canWrite refuses a member who may not work tickets.
*/
func canWrite(m authz.Membership) error {
	if !m.Can("tickets:write") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
canManage refuses a member who may not set tickets up.
*/
func canManage(m authz.Membership) error {
	if !m.Can("tickets:manage") {
		return exceptions.ErrForbidden
	}

	return nil
}
