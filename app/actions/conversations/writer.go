/*
|--------------------------------------------------------------------------
| Conversation Writer
|--------------------------------------------------------------------------
|
| Every change to a conversation is written the same way, whichever
| request makes it: the row changes, an event row joins the timeline so
| the history says who did what, and the domain event goes out, all in
| the transaction of the request. The writer is that one way.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Actor is who made a change, as the timeline names them: their type,
id and name. The customer is an actor of type contact.
*/
type Actor struct {
	Type string
	ID   ids.UUID
	Name string
}

/*
Writer writes the changes of one transaction. Now is the time they
happen, and Timezone the actor's own time zone, nil when they set none.
*/
type Writer struct {
	Q        *models.Queries
	Tx       *database.Tx
	Events   *events.Dispatcher
	Now      time.Time
	Actor    Actor
	Timezone *string
}

/*
Services are what a writer needs besides its transaction: where events
go and the clock that says when changes happen.
*/
type Services struct {
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
NewWriter returns a writer for the transaction, acting as the caller's
member, whose name it reads.
*/
func NewWriter(ctx context.Context, tx *database.Tx, s Services, m authz.Membership) (*Writer, error) {
	q := models.New(tx)

	members, err := Members(ctx, q, m.Workspace.ID, []ids.UUID{m.Member.ID})
	if err != nil {
		return nil, fmt.Errorf("load the caller: %w", err)
	}

	user := members[m.Member.ID].User
	actor := Actor{Type: string(enums.AuthorTypeMember), ID: m.Member.ID, Name: user.Name}

	return &Writer{Q: q, Tx: tx, Events: s.Events, Now: s.Clock.Now(), Actor: actor, Timezone: user.Timezone}, nil
}

/*
Dispatch sends the event within the writer's transaction.
*/
func (w *Writer) Dispatch(ctx context.Context, e events.Event) error {
	return w.Events.Dispatch(ctx, w.Tx, e)
}

/*
Record adds an event row to the conversation's timeline: the event's
name, the actor, and what the change carried.
*/
func (w *Writer) Record(ctx context.Context, c models.Conversation, name string, data map[string]any) error {
	body := map[string]any{
		"name": name, "actor": map[string]any{"type": w.Actor.Type, "id": w.actorID(), "name": w.Actor.Name},
	}
	maps.Copy(body, data)

	event, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode timeline event: %w", err)
	}

	_, err = w.Q.CreateMessage(ctx, models.CreateMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Type: string(enums.MessageTypeEvent),
		AuthorType: w.Actor.Type, AuthorID: w.authorID(), Event: event, Substantive: false,
		Metadata: json.RawMessage(`{}`), Now: w.Now,
	})
	if err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}

	return nil
}

/*
actorID returns the public id of the actor, empty for the system.
*/
func (w *Writer) actorID() string {
	switch w.Actor.Type {
	case string(enums.AuthorTypeMember):
		return models.PublicID(models.PrefixMember, w.Actor.ID)
	case string(enums.AuthorTypeContact):
		return models.PublicID(models.PrefixContact, w.Actor.ID)
	}

	return ""
}

/*
authorID returns the actor's id for an author column, nil for nobody.
*/
func (w *Writer) authorID() *ids.UUID {
	if w.Actor.ID == ids.Nil() {
		return nil
	}

	return &w.Actor.ID
}

/*
memberRef returns how the timeline names a member: their public id and
name, or nil for nobody.
*/
func memberRef(id *ids.UUID, members map[ids.UUID]Member) any {
	if id == nil {
		return nil
	}

	return map[string]any{
		"id": models.PublicID(models.PrefixMember, *id), "name": members[*id].User.Name,
	}
}

/*
teamRef returns how the timeline names a team: its public id and name,
or nil for none.
*/
func teamRef(team *models.Team) any {
	if team == nil {
		return nil
	}

	return map[string]any{"id": models.PublicID(models.PrefixTeam, team.ID), "name": team.Name}
}

/*
orNil returns the id, or ids.Nil() for none, the way events name
nobody.
*/
func orNil(id *ids.UUID) ids.UUID {
	if id == nil {
		return ids.Nil()
	}

	return *id
}
