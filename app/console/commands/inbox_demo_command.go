//go:build dev

/*
|--------------------------------------------------------------------------
| Inbox Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace with the volume a busy team lives with,
| so the inbox can be judged at scale: a team of teammates, hundreds of
| conversations through the real actions, some with threads hundreds of
| messages long, spread over weeks, open, snoozed, done and assigned
| around the team. It writes only to the workspace it is given and never
| to a release build. Every teammate it adds signs in with the password
| it prints.
|
*/

package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
demoPassword is what every seeded teammate signs in with.
*/
const demoPassword = "dimmi-demo-team-2026"

/*
InboxDemoCommand seeds a workspace with a team and a busy inbox.
*/
type InboxDemoCommand struct {
	App *app.App
}

/*
Name returns inbox:demo.
*/
func (c InboxDemoCommand) Name() string {
	return "inbox:demo"
}

/*
Description says what the command does.
*/
func (c InboxDemoCommand) Description() string {
	return "Seed a development workspace with a team and hundreds of conversations, some very long"
}

/*
Flags names the workspace, the owner the demo acts as and how much to
seed.
*/
func (c InboxDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the owner's member public id, mem_...")
	flags.Int("conversations", 400, "how many conversations to seed")
	flags.Int("teammates", 30, "how many teammates to add")
	flags.Int("long", 6, "how many conversations get threads hundreds of messages long")
}

/*
Handle seeds the workspace.
*/
func (c InboxDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	owner, err := SeriesDemoCommand(c).membership(ctx, in.String("workspace"), in.String("member"))
	if err != nil {
		return err
	}

	ctx = tenancy.With(ctx, owner.Workspace.ID.String())

	team, err := c.team(ctx, owner, in.Int("teammates"))
	if err != nil {
		return err
	}

	inbox, err := models.New(c.App.DB).FindDefaultInbox(ctx, owner.Workspace.ID)
	if err != nil {
		return fmt.Errorf("find default inbox: %w", err)
	}

	out.Line("added %d teammates; each signs in with %s", len(team)-1, demoPassword)

	total, long := in.Int("conversations"), in.Int("long")
	made := make([]ids.UUID, 0, total)

	for i := range total {
		id, err := c.conversation(ctx, team, inbox.ID, i, i < long)
		if err != nil {
			return fmt.Errorf("conversation %d: %w", i, err)
		}

		made = append(made, id)
		if (i+1)%50 == 0 {
			out.Line("seeded %d conversations", i+1)
		}
	}

	if err := c.spread(ctx, owner.Workspace.ID, made); err != nil {
		return err
	}

	out.Line("seeded %d conversations, %d of them hundreds of messages long", total, long)

	return nil
}

/*
team adds the teammates, half members and a few admins, and returns
their memberships with the owner's first.
*/
func (c InboxDemoCommand) team(ctx context.Context, owner authz.Membership, n int) ([]authz.Membership, error) {
	hash, err := c.App.Hash.Make(ctx, demoPassword)
	if err != nil {
		return nil, fmt.Errorf("hash demo password: %w", err)
	}

	q := models.New(c.App.DB)
	team := []authz.Membership{owner}

	rows, err := c.App.DB.Query(ctx, `SELECT id, role FROM workspace_members
		WHERE workspace_id = $1 AND id <> $2 AND status = 'active'`, owner.Workspace.ID, owner.Member.ID)
	if err != nil {
		return nil, fmt.Errorf("list teammates: %w", err)
	}

	type held struct {
		id   ids.UUID
		role string
	}

	var existing []held

	for rows.Next() {
		var h held
		if err := rows.Scan(&h.id, &h.role); err != nil {
			return nil, fmt.Errorf("read teammate: %w", err)
		}

		existing = append(existing, h)
	}

	rows.Close()

	for _, h := range existing {
		member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: owner.Workspace.ID, ID: h.id,
		})
		if err != nil {
			return nil, fmt.Errorf("find teammate: %w", err)
		}

		team = append(team, authz.NewMembership(owner.Workspace, member, appauth.RolePermissions(enums.Role(h.role))))
	}

	stamp := c.App.Clock.Now().Unix()

	for i := range n {
		name := demoName(i + 11)
		user, err := c.App.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
			Name: name, Email: fmt.Sprintf("%s.%d.%d@team.test", strings.ToLower(strings.Fields(name)[0]), stamp, i),
			PasswordHash: &hash, Verified: true, CreatedVia: appauth.CreatedViaRegister,
		})
		if err != nil {
			return nil, fmt.Errorf("create teammate: %w", err)
		}

		role := enums.RoleMember
		if i%8 == 0 {
			role = enums.RoleAdmin
		}

		var member models.WorkspaceMember

		err = c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			member, err = models.New(tx).CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
				WorkspaceID: owner.Workspace.ID, UserID: user.ID, Role: string(role),
				JoinedVia: new("cli"), Now: c.App.Clock.Now(),
			})

			return err
		})
		if err != nil {
			return nil, fmt.Errorf("add teammate: %w", err)
		}

		member, err = q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: owner.Workspace.ID, ID: member.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("find teammate: %w", err)
		}

		team = append(team, authz.NewMembership(owner.Workspace, member, appauth.RolePermissions(role)))
	}

	return team, nil
}

/*
conversation starts one customer's conversation and plays its thread
out: replies from a teammate, the customer answering, the odd note,
and an ending.
*/
func (c InboxDemoCommand) conversation(
	ctx context.Context, team []authz.Membership, inbox ids.UUID, i int, long bool,
) (ids.UUID, error) {
	owner := team[0]
	who := demoName(i)
	email := strings.ToLower(strings.ReplaceAll(who, " ", ".")) + fmt.Sprintf(".%d@customer.test", i)
	topic := demoTopics[i%len(demoTopics)]

	loaded, err := c.App.Actions.Inbox.CreateConversation.Handle(ctx, conversations.CreateConversationInput{
		Membership: owner, ByContact: true, InboxID: &inbox,
		Contact: contacts.Ref{WorkspaceID: owner.Workspace.ID, Email: &email, Name: &who},
		Subject: &topic.subject, Body: topic.opening,
	})
	if err != nil {
		return ids.UUID{}, fmt.Errorf("create: %w", err)
	}

	ref := conversations.Ref{ID: loaded.Conversation.ID}
	agent := team[(i*7)%len(team)]

	turns := 1 + i%6
	if long {
		turns = 120 + i*40
	}

	for t := range turns {
		if err := c.turn(ctx, agent, owner, ref, topic, t); err != nil {
			return ids.UUID{}, err
		}
	}

	return loaded.Conversation.ID, c.settle(ctx, agent, ref, i)
}

/*
turn adds one exchange: the teammate answers, sometimes with a note
first, and the customer writes back.
*/
func (c InboxDemoCommand) turn(
	ctx context.Context, agent, owner authz.Membership, ref conversations.Ref, topic demoTopic, t int,
) error {
	acts := c.App.Actions.Inbox
	if t%5 == 3 {
		_, err := acts.AddNote.Handle(ctx, messages.NoteInput{
			Membership: agent, Ref: ref, Body: fmt.Sprintf("Checked the logs (step %d). %s", t+1, topic.note),
		})
		if err != nil {
			return fmt.Errorf("note: %w", err)
		}
	}

	_, err := acts.Reply.Handle(ctx, messages.ReplyInput{
		Membership: agent, Ref: ref, Body: topic.replies[t%len(topic.replies)],
	})
	if err != nil {
		return fmt.Errorf("reply: %w", err)
	}

	_, err = acts.AddContactMessage.Handle(ctx, messages.ContactMessageInput{
		Membership: owner, Ref: ref, Body: topic.answers[t%len(topic.answers)],
	})
	if err != nil {
		return fmt.Errorf("customer message: %w", err)
	}

	return nil
}

/*
settle leaves the conversation the way a real queue looks: most done,
some open and waiting, a few snoozed, and some with nobody holding them.
*/
func (c InboxDemoCommand) settle(ctx context.Context, agent authz.Membership, ref conversations.Ref, i int) error {
	ch := conversations.Changes{}

	switch i % 10 {
	case 0, 1, 2, 3, 4:
		ch.Status = new("closed")
	case 5:
		ch.Status = new("snoozed")
		ch.SnoozePreset = conversations.PresetNextWeek
	case 6:
		ch.AssignMember = true
	}

	_, err := c.App.Actions.Inbox.ApplyChanges.Handle(ctx, conversations.ApplyChangesInput{
		Membership: agent, Ref: ref, Changes: ch,
	})
	if err != nil {
		return fmt.Errorf("settle: %w", err)
	}

	return nil
}

/*
spread moves each conversation back in time, the earliest seeded the
furthest, so the list reads like weeks of work instead of one minute.
*/
func (c InboxDemoCommand) spread(ctx context.Context, ws ids.UUID, made []ids.UUID) error {
	total := len(made)

	return c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for i, id := range made {
			back := time.Duration(total-i) * 97 * time.Minute

			_, err := tx.Exec(ctx, `UPDATE messages SET created_at = created_at - $3::interval,
				updated_at = updated_at - $3::interval, sent_at = sent_at - $3::interval
				WHERE workspace_id = $1 AND conversation_id = $2`, ws, id, back.String())
			if err != nil {
				return fmt.Errorf("spread messages: %w", err)
			}

			_, err = tx.Exec(ctx, `UPDATE conversations SET created_at = created_at - $3::interval,
				updated_at = updated_at - $3::interval,
				first_customer_message_at = first_customer_message_at - $3::interval,
				last_customer_message_at = last_customer_message_at - $3::interval,
				first_reply_at = first_reply_at - $3::interval, last_reply_at = last_reply_at - $3::interval,
				last_message_at = last_message_at - $3::interval, closed_at = closed_at - $3::interval,
				waiting_since = waiting_since - $3::interval,
				assignment_waiting_since = assignment_waiting_since - $3::interval,
				last_activity_at = last_activity_at - $3::interval
				WHERE workspace_id = $1 AND id = $2`, ws, id, back.String())
			if err != nil {
				return fmt.Errorf("spread conversation: %w", err)
			}
		}

		return nil
	})
}
