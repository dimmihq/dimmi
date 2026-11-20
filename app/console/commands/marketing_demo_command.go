//go:build dev

/*
|--------------------------------------------------------------------------
| Marketing Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace with a small, believable support team for
| the marketing site's product shots: a handful of named teammates and a
| queue of conversations written the way real customers write, through
| the messenger, email and the API. Everything goes through the real
| actions, so the screens it leaves are the product as it is. Only the
| channel a conversation came through and its timestamps are set
| directly, since the demo cannot receive real mail or messenger
| traffic. It never runs in a release build.
|
*/

package commands

import (
	"context"
	"fmt"
	"slices"
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
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MarketingDemoCommand seeds the workspace the marketing site is shot in.
*/
type MarketingDemoCommand struct {
	App *app.App
}

/*
Name returns marketing:demo.
*/
func (c MarketingDemoCommand) Name() string {
	return "marketing:demo"
}

/*
Description says what the command does.
*/
func (c MarketingDemoCommand) Description() string {
	return "Seed a development workspace with the team and conversations the marketing site's product shots show"
}

/*
Flags names the workspace and the owner the demo acts as.
*/
func (c MarketingDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the owner's member public id, mem_...")
	flags.String(
		"owner-email", "",
		"instead of --workspace and --member: create a new Lumen workspace owned by this account",
	)
}

/*
Handle seeds the workspace.
*/
func (c MarketingDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	workspace, member := in.String("workspace"), in.String("member")

	if email := in.String("owner-email"); email != "" {
		var err error

		workspace, member, err = c.createLumen(ctx, email, out)
		if err != nil {
			return err
		}
	}

	owner, err := SeriesDemoCommand(c).membership(ctx, workspace, member)
	if err != nil {
		return err
	}

	ctx = tenancy.With(ctx, owner.Workspace.ID.String())

	stage, err := c.stage(ctx, owner)
	if err != nil {
		return err
	}

	stories := marketingStories()

	// The oldest story first, so the newest lands on top of the list.
	for _, s := range slices.Backward(stories) {
		if err := c.story(ctx, stage, s); err != nil {
			return fmt.Errorf("story %q: %w", s.subject, err)
		}
	}

	if err := c.agentSetup(ctx, owner); err != nil {
		return err
	}

	out.Line("added %d teammates (password %s) and %d conversations", len(stage.team), demoPassword, len(stories))

	return nil
}

/*
stage adds the teammates and finds the inbox and channel connections
every story plays out against.
*/
func (c MarketingDemoCommand) stage(ctx context.Context, owner authz.Membership) (marketingStage, error) {
	team, err := c.team(ctx, owner)
	if err != nil {
		return marketingStage{}, err
	}

	inbox, err := models.New(c.App.DB).FindDefaultInbox(ctx, owner.Workspace.ID)
	if err != nil {
		return marketingStage{}, fmt.Errorf("find default inbox: %w", err)
	}

	channels, err := c.channels(ctx, owner.Workspace.ID, inbox.ID)
	if err != nil {
		return marketingStage{}, err
	}

	return marketingStage{owner: owner, team: team, inbox: inbox.ID, channels: channels}, nil
}

/*
createLumen makes a fresh Lumen workspace owned by the account with this
email, retiring an earlier one, and returns the public ids of the
workspace and its owner's membership.
*/
func (c MarketingDemoCommand) createLumen(
	ctx context.Context, email string, out *console.Output,
) (workspace, member string, err error) {
	user, err := models.New(c.App.DB).FindUserByEmail(ctx, strings.ToLower(email))
	if err != nil {
		return "", "", fmt.Errorf("find --owner-email: %w", err)
	}

	// Retire an earlier demo so the new one gets the /lumen address again.
	if _, err := c.App.DB.Exec(ctx, `UPDATE workspaces SET deleted_at = now(),
			slug = 'lumen-retired-' || extract(epoch FROM now())::bigint
		WHERE slug = 'lumen' AND created_by_user_id = $1`, user.ID); err != nil {
		return "", "", fmt.Errorf("retire earlier demo: %w", err)
	}

	created, err := c.App.Actions.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
		Owner: user, Name: "Lumen", JoinedVia: "cli",
	})
	if err != nil {
		return "", "", fmt.Errorf("create workspace: %w", err)
	}

	// The demo workspace skips onboarding, so shots open straight on the inbox.
	if _, err := c.App.DB.Exec(ctx, `UPDATE workspaces SET onboarding_step = 'done', onboarding_completed_at = now()
		WHERE id = $1`, created.Workspace.ID); err != nil {
		return "", "", fmt.Errorf("finish onboarding: %w", err)
	}

	workspace = models.PublicID(models.PrefixWorkspace, created.Workspace.ID)
	member = models.PublicID(models.PrefixMember, created.Owner.ID)
	out.Line("created workspace %s at /w/%s", workspace, created.Workspace.Slug)

	return workspace, member, nil
}

/*
team adds the named teammates and returns their memberships, in the
order of marketingTeammates.
*/
func (c MarketingDemoCommand) team(ctx context.Context, owner authz.Membership) ([]authz.Membership, error) {
	hash, err := c.App.Hash.Make(ctx, demoPassword)
	if err != nil {
		return nil, fmt.Errorf("hash demo password: %w", err)
	}

	q := models.New(c.App.DB)
	stamp := c.App.Clock.Now().Unix()
	names := marketingTeammates()
	team := make([]authz.Membership, 0, len(names))

	for i, name := range names {
		user, err := c.App.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
			Name: name, Email: fmt.Sprintf("%s.%d@lumen.test", strings.ToLower(strings.Fields(name)[0]), stamp),
			PasswordHash: &hash, Verified: true, CreatedVia: appauth.CreatedViaRegister,
		})
		if err != nil {
			return nil, fmt.Errorf("create teammate: %w", err)
		}

		role := enums.RoleMember
		if i == 0 {
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
channels returns a connection for each channel the stories use, adding
an email and a messenger connection to the inbox when it has none, so
each conversation shows the channel it would really have come through.
*/
func (c MarketingDemoCommand) channels(ctx context.Context, ws, inbox ids.UUID) (map[string]ids.UUID, error) {
	found := map[string]ids.UUID{}

	for _, kind := range []string{"email", "widget"} {
		var id ids.UUID

		err := c.App.DB.QueryRow(ctx, `SELECT id FROM channel_connections
			WHERE workspace_id = $1 AND inbox_id = $2 AND type = $3 AND deleted_at IS NULL
			ORDER BY created_at LIMIT 1`, ws, inbox, kind).Scan(&id)
		if err != nil {
			name := map[string]string{"email": "support@lumen.test", "widget": "Lumen messenger"}[kind]

			err = c.App.DB.QueryRow(ctx, `INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status)
				VALUES ($1, $2, $3, $4, 'active') RETURNING id`, ws, inbox, kind, name).Scan(&id)
			if err != nil {
				return nil, fmt.Errorf("add %s connection: %w", kind, err)
			}
		}

		found[kind] = id
	}

	return found, nil
}

/*
marketingStage is what every story plays out against: the owner the
demo acts as, the teammates, the inbox and a connection per channel.
*/
type marketingStage struct {
	owner    authz.Membership
	team     []authz.Membership
	inbox    ids.UUID
	channels map[string]ids.UUID
}

/*
marketingThread is one story's conversation and who speaks in it: the
owner on the customer's behalf and the teammate handling it.
*/
type marketingThread struct {
	owner, agent authz.Membership
	ref          conversations.Ref
}

/*
story plays one conversation out through the real actions, then moves
it to its channel and back in time.
*/
func (c MarketingDemoCommand) story(ctx context.Context, stage marketingStage, s marketingStory) error {
	owner, inbox := stage.owner, stage.inbox

	email := strings.ToLower(strings.ReplaceAll(s.name, " ", ".")) + "@" +
		strings.ToLower(strings.NewReplacer(" ", "", "&", "and").Replace(s.company)) + ".test"

	loaded, err := c.App.Actions.Inbox.CreateConversation.Handle(ctx, conversations.CreateConversationInput{
		Membership: owner, ByContact: true, InboxID: &inbox,
		Contact: contacts.Ref{WorkspaceID: owner.Workspace.ID, Email: &email, Name: &s.name},
		Subject: &s.subject, Body: s.turns[0].body,
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	thread := marketingThread{owner: owner, agent: owner, ref: conversations.Ref{ID: loaded.Conversation.ID}}
	if s.teammate >= 0 {
		thread.agent = stage.team[s.teammate]
	}

	for _, turn := range s.turns[1:] {
		if err := c.turn(ctx, thread, turn); err != nil {
			return fmt.Errorf("%s turn: %w", turn.by, err)
		}
	}

	if err := c.settle(ctx, thread, s); err != nil {
		return err
	}

	return c.place(ctx, marketingPlacement{
		workspace: owner.Workspace.ID, conversation: loaded.Conversation.ID,
		channel: stage.channels[s.channel], ago: s.ago,
	})
}

/*
turn adds one message after the first: a teammate's reply or note, or
the customer writing again.
*/
func (c MarketingDemoCommand) turn(ctx context.Context, t marketingThread, turn marketingTurn) error {
	acts := c.App.Actions.Inbox
	owner, agent, ref := t.owner, t.agent, t.ref

	var err error

	switch turn.by {
	case "agent":
		_, err = acts.Reply.Handle(ctx, messages.ReplyInput{Membership: agent, Ref: ref, Body: turn.body})
	case "note":
		_, err = acts.AddNote.Handle(ctx, messages.NoteInput{Membership: agent, Ref: ref, Body: turn.body})
	default:
		_, err = acts.AddContactMessage.Handle(ctx, messages.ContactMessageInput{
			Membership: owner, Ref: ref, Body: turn.body,
		})
	}

	return err
}

/*
settle leaves the conversation where its story says: closed, snoozed or
open, and assigned to its teammate.
*/
func (c MarketingDemoCommand) settle(ctx context.Context, t marketingThread, s marketingStory) error {
	ch := conversations.Changes{}

	switch s.status {
	case "closed":
		ch.Status = new("closed")
	case "snoozed":
		ch.Status = new("snoozed")
		ch.SnoozePreset = conversations.PresetNextWeek
	}

	// A teammate's reply already assigns the conversation to them; assigning
	// again would hand it back, so only stories without a reply assign here.
	replied := false
	for _, turn := range s.turns {
		replied = replied || turn.by == "agent"
	}

	if s.teammate >= 0 && !replied {
		ch.AssignMember = true
	}

	if ch.Status == nil && !ch.AssignMember {
		return nil
	}

	in := conversations.ApplyChangesInput{Membership: t.agent, Ref: t.ref, Changes: ch}
	if _, err := c.App.Actions.Inbox.ApplyChanges.Handle(ctx, in); err != nil {
		return fmt.Errorf("settle: %w", err)
	}

	return nil
}

/*
marketingPlacement says where a story's conversation goes: its channel
and how long ago it last moved.
*/
type marketingPlacement struct {
	workspace, conversation, channel ids.UUID
	ago                              time.Duration
}

/*
place moves a conversation to the channel its story names and back in
time, so the list reads like a real morning: the last message p.ago ago
and each earlier one a few minutes before it.
*/
func (c MarketingDemoCommand) place(ctx context.Context, p marketingPlacement) error {
	ws, id, channel, ago := p.workspace, p.conversation, p.channel, p.ago

	return c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if channel != (ids.UUID{}) {
			if _, err := tx.Exec(ctx, `UPDATE conversations SET channel_connection_id = $3
				WHERE workspace_id = $1 AND id = $2`, ws, id, channel); err != nil {
				return fmt.Errorf("move channel: %w", err)
			}
		}

		// Rewrite each message's time: the newest is ago old, each earlier one 4 minutes older.
		if _, err := tx.Exec(ctx, `WITH ordered AS (
				SELECT id, row_number() OVER (ORDER BY created_at DESC) - 1 AS back FROM messages
				WHERE workspace_id = $1 AND conversation_id = $2)
			UPDATE messages m SET
				created_at = now() - $3::interval - (o.back * interval '4 minutes'),
				updated_at = now() - $3::interval - (o.back * interval '4 minutes'),
				sent_at = CASE WHEN m.sent_at IS NULL THEN NULL
					ELSE now() - $3::interval - (o.back * interval '4 minutes') END
			FROM ordered o WHERE m.id = o.id`, ws, id, ago.String()); err != nil {
			return fmt.Errorf("move messages: %w", err)
		}

		shift := `now() - $3::interval`
		_, err := tx.Exec(ctx, `UPDATE conversations SET
				updated_at = `+shift+`, last_message_at = `+shift+`, last_activity_at = `+shift+`,
				created_at = LEAST(created_at, (SELECT min(created_at) FROM messages
					WHERE workspace_id = $1 AND conversation_id = $2)),
				first_customer_message_at = (SELECT min(created_at) FROM messages
					WHERE workspace_id = $1 AND conversation_id = $2),
				waiting_since = CASE WHEN waiting_since IS NULL THEN NULL ELSE `+shift+` END
			WHERE workspace_id = $1 AND id = $2`, ws, id, ago.String())
		if err != nil {
			return fmt.Errorf("move conversation: %w", err)
		}

		return nil
	})
}
