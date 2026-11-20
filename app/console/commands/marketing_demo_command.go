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
	flags.String("owner-email", "", "instead of --workspace and --member: create a new Lumen workspace owned by this account")
}

/*
marketingTeammates are the people on the demo team, in the order the
stories hand conversations to them.
*/
var marketingTeammates = []string{"Priya Moreau", "Kofi Mensah", "Lena Fischer", "Mateo Reyes", "Hana Tanaka"}

/*
marketingTurn is one message in a story: who says it and what.
*/
type marketingTurn struct {
	by   string // "customer", "agent" or "note"
	body string
}

/*
marketingStory is one conversation: who wrote in, through which
channel, what about, how it went, where it stands and how long ago it
last moved.
*/
type marketingStory struct {
	name, company, channel, subject string
	turns                           []marketingTurn
	status                          string // open, closed, snoozed
	teammate                        int    // index into marketingTeammates, -1 for nobody
	ago                             time.Duration
}

var marketingStories = []marketingStory{
	{
		name: "Omar Haddad", company: "Northwind Labs", channel: "email", subject: "Webhook deliveries failing since 9am",
		turns: []marketingTurn{
			{"customer", "Hi team, every webhook to our endpoint has failed since about 9:00 UTC. Nothing changed on our side. Can you check what you see?"},
			{"note", "Their endpoint answers 301 to our requests since they moved to a new domain. Our sender doesn't follow redirects."},
			{"agent", "Thanks Omar, I can see it. Since 08:57 UTC your endpoint answers every delivery with a 301 redirect to api.northwind.dev, and webhooks don't follow redirects. If you update the endpoint URL in Settings → Webhooks, the failed deliveries will retry on their own within the hour."},
			{"customer", "That's it, we moved domains last night. Updated now, I can see deliveries coming through again. Thank you!"},
		},
		status: "open", teammate: 0, ago: 3 * time.Minute,
	},
	{
		name: "Hana Kowalski", company: "Fieldnote", channel: "widget", subject: "Adding a teammate to our workspace",
		turns: []marketingTurn{
			{"customer", "How do I add a new teammate? I can't find the invite button."},
			{"agent", "Open Settings, then Team, and choose Invite. They get an email link that's good for 7 days."},
		},
		status: "open", teammate: 1, ago: 11 * time.Minute,
	},
	{
		name: "Sven Larsen", company: "Brightwave", channel: "email", subject: "SSO broken after certificate rotation",
		turns: []marketingTurn{
			{"customer", "We rotated our SAML signing certificate this morning and now nobody on the team can sign in."},
		},
		status: "open", teammate: -1, ago: 24 * time.Minute,
	},
	{
		name: "Ada Okafor", company: "Pinecrest", channel: "widget", subject: "Charged twice for November",
		turns: []marketingTurn{
			{"customer", "Looks like we were charged twice for November. Can you refund one of them?"},
			{"agent", "You're right, there are two $49.00 charges on November 3. I've refunded the duplicate; it shows on your card in 5 to 10 days."},
			{"customer", "Perfect, thanks for the quick fix."},
		},
		status: "closed", teammate: 2, ago: 38 * time.Minute,
	},
	{
		name: "Mateo Silva", company: "Larkspur", channel: "api", subject: "Rate limits on the contacts API",
		turns: []marketingTurn{
			{"customer", "We're importing about 80k contacts and keep hitting 429s. What's the limit and is there a bulk endpoint?"},
		},
		status: "open", teammate: 3, ago: 52 * time.Minute,
	},
	{
		name: "Yara Ibrahim", company: "Quill & Co", channel: "email", subject: "Export all conversations to CSV?",
		turns: []marketingTurn{
			{"customer", "Is there a way to export every conversation from last quarter to CSV for our QA review?"},
			{"agent", "Not as a CSV yet, but it's on our board. I've added your vote, and you'll get an email as soon as it ships."},
		},
		status: "snoozed", teammate: 4, ago: 75 * time.Minute,
	},
	{
		name: "Wes Novak", company: "Tidewater", channel: "widget", subject: "Messenger not showing on mobile",
		turns: []marketingTurn{
			{"customer", "The chat bubble shows on desktop but not on our mobile site. Any ideas?"},
			{"agent", "Check Settings → Messenger → Visibility: mobile is switched off for your site. Turn it on and it appears right away."},
		},
		status: "closed", teammate: 0, ago: 2 * time.Hour,
	},
	{
		name: "Tara Mensah", company: "Oakline", channel: "email", subject: "Custom domain for our help center",
		turns: []marketingTurn{
			{"customer", "We'd like our help center on help.oakline.io. What DNS records do we need?"},
		},
		status: "open", teammate: 1, ago: 3 * time.Hour,
	},
	{
		name: "Eli Nakamura", company: "Northwind Labs", channel: "api", subject: "Identity verification with Next.js",
		turns: []marketingTurn{
			{"customer", "Following the Next.js guide, but the messenger still shows visitors as anonymous after sign-in."},
			{"agent", "Your token route returns the token as plain text; the messenger expects JSON like {\"token\": \"...\"}. Wrapping it fixes it."},
		},
		status: "closed", teammate: 3, ago: 5 * time.Hour,
	},
	{
		name: "Rosa Costa", company: "Fieldnote", channel: "widget", subject: "Office hours in two time zones",
		turns: []marketingTurn{
			{"customer", "Our team is split between Lisbon and Toronto. Can office hours cover both?"},
		},
		status: "open", teammate: -1, ago: 7 * time.Hour,
	},
	{
		name: "Ivan Adeyemi", company: "Brightwave", channel: "email", subject: "Upgrade to annual billing",
		turns: []marketingTurn{
			{"customer", "We'd like to move to annual billing from next month. Can you set that up?"},
			{"agent", "Done. Annual billing starts on your next renewal, December 1, and your invoice goes to billing@brightwave.com."},
		},
		status: "closed", teammate: 2, ago: 26 * time.Hour,
	},
	{
		name: "Nia Lindqvist", company: "Pinecrest", channel: "widget", subject: "Translate articles into German",
		turns: []marketingTurn{
			{"customer", "Can our help center serve German versions of the articles?"},
		},
		status: "open", teammate: 4, ago: 30 * time.Hour,
	},
}

/*
Handle seeds the workspace.
*/
func (c MarketingDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	workspace, member := in.String("workspace"), in.String("member")

	if email := in.String("owner-email"); email != "" {
		user, err := models.New(c.App.DB).FindUserByEmail(ctx, strings.ToLower(email))
		if err != nil {
			return fmt.Errorf("find --owner-email: %w", err)
		}

		// Retire an earlier demo so the new one gets the /lumen address again.
		if _, err := c.App.DB.Exec(ctx, `UPDATE workspaces SET deleted_at = now(),
				slug = 'lumen-retired-' || extract(epoch FROM now())::bigint
			WHERE slug = 'lumen' AND created_by_user_id = $1`, user.ID); err != nil {
			return fmt.Errorf("retire earlier demo: %w", err)
		}

		created, err := c.App.Actions.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
			Owner: user, Name: "Lumen", JoinedVia: "cli",
		})
		if err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}

		// The demo workspace skips onboarding, so shots open straight on the inbox.
		if _, err := c.App.DB.Exec(ctx, `UPDATE workspaces SET onboarding_step = 'done', onboarding_completed_at = now()
			WHERE id = $1`, created.Workspace.ID); err != nil {
			return fmt.Errorf("finish onboarding: %w", err)
		}

		workspace = models.PublicID(models.PrefixWorkspace, created.Workspace.ID)
		member = models.PublicID(models.PrefixMember, created.Owner.ID)
		out.Line("created workspace %s at /w/%s", workspace, created.Workspace.Slug)
	}

	owner, err := SeriesDemoCommand(c).membership(ctx, workspace, member)
	if err != nil {
		return err
	}

	ctx = tenancy.With(ctx, owner.Workspace.ID.String())

	team, err := c.team(ctx, owner)
	if err != nil {
		return err
	}

	inbox, err := models.New(c.App.DB).FindDefaultInbox(ctx, owner.Workspace.ID)
	if err != nil {
		return fmt.Errorf("find default inbox: %w", err)
	}

	channels, err := c.channels(ctx, owner.Workspace.ID, inbox.ID)
	if err != nil {
		return err
	}

	// The oldest story first, so the newest lands on top of the list.
	for i := len(marketingStories) - 1; i >= 0; i-- {
		if err := c.story(ctx, owner, team, inbox.ID, channels, marketingStories[i]); err != nil {
			return fmt.Errorf("story %q: %w", marketingStories[i].subject, err)
		}
	}

	if err := c.agentSetup(ctx, owner); err != nil {
		return err
	}

	out.Line("added %d teammates (password %s) and %d conversations", len(team), demoPassword, len(marketingStories))

	return nil
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
	team := make([]authz.Membership, 0, len(marketingTeammates))

	for i, name := range marketingTeammates {
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
story plays one conversation out through the real actions, then moves
it to its channel and back in time.
*/
func (c MarketingDemoCommand) story(
	ctx context.Context, owner authz.Membership, team []authz.Membership, inbox ids.UUID,
	channels map[string]ids.UUID, s marketingStory,
) error {
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

	ref := conversations.Ref{ID: loaded.Conversation.ID}

	agent := owner
	if s.teammate >= 0 {
		agent = team[s.teammate]
	}

	acts := c.App.Actions.Inbox

	for _, turn := range s.turns[1:] {
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

		if err != nil {
			return fmt.Errorf("%s turn: %w", turn.by, err)
		}
	}

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

	if ch.Status != nil || ch.AssignMember {
		_, err = acts.ApplyChanges.Handle(ctx, conversations.ApplyChangesInput{Membership: agent, Ref: ref, Changes: ch})
		if err != nil {
			return fmt.Errorf("settle: %w", err)
		}
	}

	return c.place(ctx, owner.Workspace.ID, loaded.Conversation.ID, channels[s.channel], s.ago)
}

/*
place moves a conversation to the channel its story names and back in
time, so the list reads like a real morning: the last message s.ago ago
and each earlier one a few minutes before it.
*/
func (c MarketingDemoCommand) place(ctx context.Context, ws, id, channel ids.UUID, ago time.Duration) error {
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
				sent_at = CASE WHEN m.sent_at IS NULL THEN NULL ELSE now() - $3::interval - (o.back * interval '4 minutes') END
			FROM ordered o WHERE m.id = o.id`, ws, id, ago.String()); err != nil {
			return fmt.Errorf("move messages: %w", err)
		}

		shift := `now() - $3::interval`
		_, err := tx.Exec(ctx, `UPDATE conversations SET
				updated_at = `+shift+`, last_message_at = `+shift+`, last_activity_at = `+shift+`,
				created_at = LEAST(created_at, (SELECT min(created_at) FROM messages WHERE workspace_id = $1 AND conversation_id = $2)),
				first_customer_message_at = (SELECT min(created_at) FROM messages WHERE workspace_id = $1 AND conversation_id = $2),
				waiting_since = CASE WHEN waiting_since IS NULL THEN NULL ELSE `+shift+` END
			WHERE workspace_id = $1 AND id = $2`, ws, id, ago.String())
		if err != nil {
			return fmt.Errorf("move conversation: %w", err)
		}

		return nil
	})
}
