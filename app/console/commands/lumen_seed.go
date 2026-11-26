//go:build dev

/*
|--------------------------------------------------------------------------
| Lumen Seed
|--------------------------------------------------------------------------
|
| Seeds Lumen, the second development workspace produced by `db:seed`: a
| small, curated support team with believable conversations across
| messenger, email and the API, plus the billing actions, versioned refund
| procedure, saved simulations and triage workflow used for demos and
| marketing/docs screenshots. Everything goes through the real actions, so
| the screens it leaves are the product as it is.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
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
	"github.com/dimmihq/dimmi/app/demo/stowline"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

const (
	lumenName         = "Lumen"
	lumenSlug         = "lumen"
	lumenDefaultOwner = "maya@lumen.test"
)

/*
lumenSeeder seeds the Lumen demo workspace.
*/
type lumenSeeder struct {
	App        *app.App
	Reset      bool
	OwnerEmail string
	Password   string
	Out        *console.Output
}

/*
run creates and populates the Lumen workspace.
*/
func (s lumenSeeder) run(ctx context.Context) error {
	began := s.App.Clock.Now()
	if err := stowline.ClearSeeded(ctx, s.App, stowline.ClearOptions{
		Slug: lumenSlug, Label: lumenName, Reset: s.Reset, Log: s.Out.Line,
	}); err != nil {
		return err
	}

	ownerEmail := s.OwnerEmail
	if ownerEmail == "" {
		ownerEmail = lumenDefaultOwner
	}

	owner, hash, err := s.createLumen(ctx, ownerEmail)
	if err != nil {
		return err
	}

	ctx = tenancy.With(ctx, owner.Workspace.ID.String())

	stage, err := s.stage(ctx, owner, hash)
	if err != nil {
		return err
	}

	stories := marketingStories()

	// The oldest story first, so the newest lands on top of the list.
	for _, st := range slices.Backward(stories) {
		if err := s.story(ctx, stage, st); err != nil {
			return fmt.Errorf("story %q: %w", st.subject, err)
		}
	}

	if err := s.agentSetup(ctx, owner); err != nil {
		return err
	}

	s.Out.Info("Lumen is ready at /w/%s after %s (%d teammates, %d conversations). Sign in as %s with %q.",
		owner.Workspace.Slug, s.App.Clock.Now().Sub(began).Round(time.Second), len(stage.team)+1, len(stories),
		ownerEmail, s.Password)

	return nil
}

/*
findOrCreateUser returns the account with email, creating it with name and
password hash when absent.
*/
func (s lumenSeeder) findOrCreateUser(ctx context.Context, name, email, hash string) (models.User, error) {
	u, err := models.New(s.App.DB).FindUserByEmail(ctx, strings.ToLower(email))
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return u, fmt.Errorf("find user %s: %w", email, err)
	}
	return s.App.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
		Name: name, Email: email, PasswordHash: &hash, Verified: true, CreatedVia: appauth.CreatedViaRegister,
	})
}

/*
createLumen makes a fresh Lumen workspace owned by email, marks it as seeded
so --reset can purge only it later, and skips onboarding so the inbox opens
immediately.
*/
func (s lumenSeeder) createLumen(ctx context.Context, email string) (authz.Membership, string, error) {
	hash, err := s.App.Hash.Make(ctx, s.Password)
	if err != nil {
		return authz.Membership{}, "", fmt.Errorf("hash password: %w", err)
	}

	user, err := s.findOrCreateUser(ctx, "Maya Lin", email, hash)
	if err != nil {
		return authz.Membership{}, "", fmt.Errorf("owner %s: %w", email, err)
	}

	created, err := s.App.Actions.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
		Owner: user, Name: lumenName, JoinedVia: "cli",
	})
	if err != nil {
		return authz.Membership{}, "", fmt.Errorf("create workspace: %w", err)
	}

	if _, err := s.App.DB.Exec(ctx, `UPDATE workspaces SET settings = settings || jsonb_build_object('seed', $2::text),
			onboarding_step = 'done', onboarding_completed_at = now()
		WHERE id = $1`, created.Workspace.ID, lumenSlug); err != nil {
		return authz.Membership{}, "", fmt.Errorf("mark workspace: %w", err)
	}

	scoped := tenancy.With(ctx, created.Workspace.ID.String())
	q := models.New(s.App.DB)
	w, err := q.FindWorkspace(scoped, created.Workspace.ID)
	if err != nil {
		return authz.Membership{}, "", fmt.Errorf("reload workspace: %w", err)
	}
	member, err := q.FindWorkspaceMember(scoped, models.FindWorkspaceMemberParams{
		WorkspaceID: w.ID, ID: created.Owner.ID,
	})
	if err != nil {
		return authz.Membership{}, "", fmt.Errorf("find owner: %w", err)
	}

	return authz.NewMembership(w, member, appauth.RolePermissions(enums.RoleOwner)), hash, nil
}

/*
stage adds the teammates and finds the inbox and channel connections
every story plays out against.
*/
func (s lumenSeeder) stage(ctx context.Context, owner authz.Membership, hash string) (marketingStage, error) {
	team, err := s.team(ctx, owner, hash)
	if err != nil {
		return marketingStage{}, err
	}

	inbox, err := models.New(s.App.DB).FindDefaultInbox(ctx, owner.Workspace.ID)
	if err != nil {
		return marketingStage{}, fmt.Errorf("find default inbox: %w", err)
	}

	channels, err := s.channels(ctx, owner.Workspace.ID, inbox.ID)
	if err != nil {
		return marketingStage{}, err
	}

	return marketingStage{owner: owner, team: team, inbox: inbox.ID, channels: channels}, nil
}

/*
team adds the named teammates and returns their memberships, in the
order of marketingTeammates.
*/
func (s lumenSeeder) team(ctx context.Context, owner authz.Membership, hash string) ([]authz.Membership, error) {
	q := models.New(s.App.DB)
	names := marketingTeammates()
	team := make([]authz.Membership, 0, len(names))

	for i, name := range names {
		email := fmt.Sprintf("%s@lumen.test", strings.ToLower(strings.Fields(name)[0]))
		user, err := s.findOrCreateUser(ctx, name, email, hash)
		if err != nil {
			return nil, fmt.Errorf("create teammate: %w", err)
		}

		role := enums.RoleMember
		if i == 0 {
			role = enums.RoleAdmin
		}

		var member models.WorkspaceMember
		err = s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			member, err = models.New(tx).CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
				WorkspaceID: owner.Workspace.ID, UserID: user.ID, Role: string(role),
				JoinedVia: new("cli"), Now: s.App.Clock.Now(),
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
func (s lumenSeeder) channels(ctx context.Context, ws, inbox ids.UUID) (map[string]ids.UUID, error) {
	found := map[string]ids.UUID{}

	for _, kind := range []string{"email", "widget"} {
		var id ids.UUID

		err := s.App.DB.QueryRow(ctx, `SELECT id FROM channel_connections
			WHERE workspace_id = $1 AND inbox_id = $2 AND type = $3 AND deleted_at IS NULL
			ORDER BY created_at LIMIT 1`, ws, inbox, kind).Scan(&id)
		if err != nil {
			name := map[string]string{"email": "support@lumen.test", "widget": "Lumen messenger"}[kind]

			err = s.App.DB.QueryRow(ctx, `INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status)
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
func (s lumenSeeder) story(ctx context.Context, stage marketingStage, st marketingStory) error {
	owner, inbox := stage.owner, stage.inbox

	email := strings.ToLower(strings.ReplaceAll(st.name, " ", ".")) + "@" +
		strings.ToLower(strings.NewReplacer(" ", "", "&", "and").Replace(st.company)) + ".test"

	loaded, err := s.App.Actions.Inbox.CreateConversation.Handle(ctx, conversations.CreateConversationInput{
		Membership: owner, ByContact: true, InboxID: &inbox,
		Contact: contacts.Ref{WorkspaceID: owner.Workspace.ID, Email: &email, Name: &st.name},
		Subject: &st.subject, Body: st.turns[0].body,
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	thread := marketingThread{owner: owner, agent: owner, ref: conversations.Ref{ID: loaded.Conversation.ID}}
	if st.teammate >= 0 {
		thread.agent = stage.team[st.teammate]
	}

	for _, turn := range st.turns[1:] {
		if err := s.turn(ctx, thread, turn); err != nil {
			return fmt.Errorf("%s turn: %w", turn.by, err)
		}
	}

	if err := s.settle(ctx, thread, st); err != nil {
		return err
	}

	return s.place(ctx, marketingPlacement{
		workspace: owner.Workspace.ID, conversation: loaded.Conversation.ID,
		channel: stage.channels[st.channel], ago: st.ago,
	})
}

/*
turn adds one message after the first: a teammate's reply or note, or
the customer writing again.
*/
func (s lumenSeeder) turn(ctx context.Context, t marketingThread, turn marketingTurn) error {
	acts := s.App.Actions.Inbox
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
func (s lumenSeeder) settle(ctx context.Context, t marketingThread, st marketingStory) error {
	ch := conversations.Changes{}

	switch st.status {
	case "closed":
		ch.Status = new("closed")
	case "snoozed":
		ch.Status = new("snoozed")
		ch.SnoozePreset = conversations.PresetNextWeek
	}

	// A teammate's reply already assigns the conversation to them; assigning
	// again would hand it back, so only stories without a reply assign here.
	replied := false
	for _, turn := range st.turns {
		replied = replied || turn.by == "agent"
	}

	if st.teammate >= 0 && !replied {
		ch.AssignMember = true
	}

	if ch.Status == nil && !ch.AssignMember {
		return nil
	}

	in := conversations.ApplyChangesInput{Membership: t.agent, Ref: t.ref, Changes: ch}
	if _, err := s.App.Actions.Inbox.ApplyChanges.Handle(ctx, in); err != nil {
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
func (s lumenSeeder) place(ctx context.Context, p marketingPlacement) error {
	ws, id, channel, ago := p.workspace, p.conversation, p.channel, p.ago

	return s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
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
