/*
|--------------------------------------------------------------------------
| Incidents
|--------------------------------------------------------------------------
|
| An incident is a tracker for an outage. Beyond what a tracker does, it
| keeps a public log of short posts customers read on the messenger's
| status card and the help center's status page, each of which may also
| go to every linked customer. Declaring one from the inbox links the
| chosen conversations at once. While it is active, new conversations
| the team or the detector match to it are linked and told the latest.
|
*/

package tickets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
recentIncidents is how long a resolved incident stays on status pages.
*/
const recentIncidents = 7 * 24 * time.Hour

/*
PublicUpdateInput is a post to an incident's public log. StateID moves
the incident with the post; SendToLinked also messages every linked
customer.
*/
type PublicUpdateInput struct {
	Body         string
	StateID      *ids.UUID
	SendToLinked bool
	Source       string
}

/*
PostPublicUpdate adds to an incident's public log.
*/
func (t *Tickets) PostPublicUpdate(
	ctx context.Context, m authz.Membership, ref Ref, in PublicUpdateInput,
) (models.TicketPublicUpdate, error) {
	if err := canWrite(m); err != nil {
		return models.TicketPublicUpdate{}, err
	}

	var out models.TicketPublicUpdate

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := find(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		locked, err := w.lockRow(ctx, m.Workspace.ID, found.ID)
		if err != nil {
			return err
		}

		out, err = w.PublicUpdate(ctx, locked, in)

		return err
	})

	return out, err
}

/*
PublicUpdate posts to the incident's log, moving its state and
messaging linked customers when asked.
*/
func (w *Writer) PublicUpdate(
	ctx context.Context, t models.Ticket, in PublicUpdateInput,
) (models.TicketPublicUpdate, error) {
	if t.Kind != KindIncident {
		return models.TicketPublicUpdate{}, ErrNotTracker
	}

	in.Body = strings.TrimSpace(in.Body)
	if n := len([]rune(in.Body)); n < 1 || n > 2000 {
		return models.TicketPublicUpdate{}, ErrNoteInvalid
	}

	if in.StateID != nil {
		if _, err := w.Apply(ctx, t, Change{StateID: in.StateID}); err != nil {
			return models.TicketPublicUpdate{}, err
		}
	}

	var updateID *ids.UUID

	if in.SendToLinked {
		plan, err := w.planUpdate(ctx, t, UpdateInput{Body: in.Body})
		if err != nil {
			return models.TicketPublicUpdate{}, err
		}

		updateID = &plan.Update.ID
	}

	return w.storePublic(ctx, t, in, updateID)
}

/*
storePublic stores the post and announces it.
*/
func (w *Writer) storePublic(
	ctx context.Context, t models.Ticket, in PublicUpdateInput, updateID *ids.UUID,
) (models.TicketPublicUpdate, error) {
	var author *ids.UUID
	if w.Actor.Type == string(enums.AuthorTypeMember) {
		author = w.actorID()
	}

	source := in.Source
	if source == "" {
		source = "dimmi"
	}

	post, err := w.Q.CreateTicketPublicUpdate(ctx, models.CreateTicketPublicUpdateParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, Body: in.Body, StateID: in.StateID, UpdateID: updateID,
		AuthorMemberID: author, Source: source, Now: w.Now,
	})
	if err != nil {
		return post, fmt.Errorf("post public update: %w", err)
	}

	data := map[string]any{"body": in.Body, "id": models.PublicID(models.PrefixTicketPublicUpdate, post.ID)}
	if err := w.log(ctx, t, "public_update", data); err != nil {
		return post, err
	}

	changes := map[string]appevents.TicketChange{"public_update": change(nil, data)}

	return post, w.emit(ctx, appevents.TicketPublicUpdatePosted{TicketChanged: w.changed(t, changes)})
}

/*
PublicUpdates returns an incident's public log.
*/
func (t *Tickets) PublicUpdates(ctx context.Context, m authz.Membership, ref Ref) ([]models.TicketPublicUpdate, error) {
	if err := canRead(m); err != nil {
		return nil, err
	}

	var out []models.TicketPublicUpdate

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		out, err = q.ListTicketPublicUpdates(ctx, models.ListTicketPublicUpdatesParams{
			WorkspaceID: m.Workspace.ID, TicketID: found.ID,
		})

		return err
	})

	return out, err
}

/*
StatusIncident is an incident as status pages show it: its title, the
customer label of its state, severity and areas, and its public log.
*/
type StatusIncident struct {
	Ticket  models.Ticket
	State   models.TicketState
	Updates []models.TicketPublicUpdate
}

/*
LiveIncidents returns the customer-visible incidents active now, newest
first, with their public logs.
*/
func (t *Tickets) LiveIncidents(ctx context.Context, ws ids.UUID) ([]StatusIncident, error) {
	all, err := t.StatusIncidents(ctx, ws)

	live := all[:0]
	for _, inc := range all {
		if inc.Ticket.StateGroup != GroupResolved {
			live = append(live, inc)
		}
	}

	return live, err
}

/*
StatusIncidents returns the customer-visible incidents active now or
resolved in the last week, newest first, with their public logs.
*/
func (t *Tickets) StatusIncidents(ctx context.Context, ws ids.UUID) ([]StatusIncident, error) {
	var out []StatusIncident

	ctx = tenancy.With(ctx, ws.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		now := t.Clock.Now()

		rows, err := q.ListRecentIncidents(ctx, models.ListRecentIncidentsParams{
			WorkspaceID: ws, Since: new(now.Add(-recentIncidents)),
		})
		if err != nil {
			return fmt.Errorf("list status incidents: %w", err)
		}

		setup, err := loadSetup(ctx, q, ws, now)
		if err != nil {
			return err
		}

		for _, r := range rows {
			st, _ := setup.State(r.StateID)

			updates, err := q.ListTicketPublicUpdates(ctx, models.ListTicketPublicUpdatesParams{
				WorkspaceID: ws, TicketID: r.ID,
			})
			if err != nil {
				return fmt.Errorf("list public updates: %w", err)
			}

			out = append(out, StatusIncident{Ticket: r, State: st, Updates: updates})
		}

		return nil
	})

	return out, err
}

/*
ActiveIncidents returns the workspace's open incidents with how many
conversations each reaches, for the inbox sidebar and the AI agent.
*/
func (t *Tickets) ActiveIncidents(ctx context.Context, ws ids.UUID) ([]Summary, error) {
	var out []Summary

	ctx = tenancy.With(ctx, ws.String())

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListActiveIncidents(ctx, ws)
		if err != nil {
			return fmt.Errorf("list active incidents: %w", err)
		}

		out, err = summarize(ctx, q, rows, t.Clock.Now())

		return err
	})

	return out, err
}

/*
DeclareInput declares an incident: its type, title, fields and the
conversations already reporting it.
*/
type DeclareInput struct {
	TypeID        *ids.UUID
	Title         string
	Description   string
	Fields        map[string]any
	Conversations []ids.UUID
}

/*
Declare makes an incident and links the conversations at once.
*/
func (w *Writer) Declare(ctx context.Context, ws ids.UUID, in DeclareInput) (models.Ticket, error) {
	typeID := in.TypeID
	if typeID == nil {
		ty, ok := w.Setup.FirstType(KindIncident)
		if !ok {
			return models.Ticket{}, ErrTypeNotFound
		}

		typeID = &ty.ID
	}

	links := make([]LinkInput, len(in.Conversations))
	for i := range in.Conversations {
		links[i] = LinkInput{Kind: LinkTracker, ConversationID: &in.Conversations[i]}
	}

	made, err := w.Create(ctx, ws, CreateInput{
		TypeID: *typeID, Title: in.Title, Description: in.Description, Fields: in.Fields, Links: links,
	})

	return made.Ticket, err
}
