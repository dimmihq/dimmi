/*
|--------------------------------------------------------------------------
| Seeder
|--------------------------------------------------------------------------
|
| Writes a planned Stowline workspace into the database. The setup a
| team clicks through, its people, teams, inboxes, tags, saved replies,
| help center, AI agent, actions and procedures, goes through the same
| actions the dashboard calls, so it obeys the same rules. The year of
| history, thousands of companies, people, conversations, messages,
| ratings and past AI runs, goes in with COPY in batches, through the
| same triggers live writes fire, so reports read it as they would read
| real traffic.
|
| It never runs in production. It finds an earlier Stowline workspace by
| the mark it leaves in the workspace's settings and refuses to touch it
| unless asked to reset, which purges that one workspace for good and
| nothing else. It never calls a model or an embeddings provider: the
| help center is indexed for keyword search, and building vectors is a
| separate step.
|
*/

package stowline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrProduction refuses to seed a production install.
*/
var ErrProduction = errors.New("the Stowline seed never runs in production")

/*
ErrExists refuses to seed over an earlier Stowline workspace.
*/
var ErrExists = errors.New("a Stowline workspace already exists; run again with --reset to purge it and seed afresh")

/*
DefaultPassword is what every seeded teammate signs in with.
*/
const DefaultPassword = "stowline-demo-2026" //nolint:gosec // dev-only seeded teammates, never a real secret

/*
Seeder writes one Stowline workspace.
*/
type Seeder struct {
	App        *app.App
	Size       Size
	Seed       uint64
	Reset      bool
	OwnerEmail string
	Password   string
	Log        func(format string, args ...any)
}

/*
Result is what a run wrote: the workspace, the owner's membership, the
team's, the plan it followed, row counts per table and how long it took.
*/
type Result struct {
	Workspace models.Workspace
	Owner     authz.Membership
	Team      []authz.Membership
	Plan      Plan
	Counts    map[string]int64
	Took      time.Duration
}

/*
run is one seed in progress: what it has made so far.
*/
type run struct {
	s        *Seeder
	plan     Plan
	ws       models.Workspace
	owner    authz.Membership
	team     []authz.Membership
	teams    map[string]ids.UUID
	inboxes  map[string]ids.UUID
	channels map[string]map[string]ids.UUID
	tags     map[string]ids.UUID
	center   ids.UUID
	articles map[string]ids.UUID
	followed map[string]*followedProcedure
	agents   map[string]ids.UUID
	actions  map[string]string
}

/*
Run seeds the workspace and returns what it wrote.
*/
func (s *Seeder) Run(ctx context.Context) (*Result, error) {
	started := s.App.Clock.Now()
	if s.App.Application.Config.App.Env == foundation.EnvProduction {
		return nil, ErrProduction
	}
	if s.Log == nil {
		s.Log = func(string, ...any) {}
	}
	if s.Password == "" {
		s.Password = DefaultPassword
	}
	if err := s.clear(ctx); err != nil {
		return nil, err
	}

	r := &run{s: s, plan: NewPlan(s.Size, s.Seed, s.App.Clock.Now())}
	s.Log("planned %d companies, %d people and %d conversations", len(r.plan.Companies), len(r.plan.Contacts),
		len(r.plan.Conversations))
	if err := r.steps(ctx); err != nil {
		return nil, err
	}
	counts, err := r.counts(ctx)
	if err != nil {
		return nil, err
	}
	return &Result{
		Workspace: r.ws, Owner: r.owner, Team: r.team, Plan: r.plan, Counts: counts,
		Took: s.App.Clock.Now().Sub(started),
	}, nil
}

/*
steps runs each step of the seed in turn. The first creates the
workspace; each later one runs in one transaction inside it, so
row-level security sees the tenant for every statement, and the
actions it calls nest as savepoints.
*/
func (r *run) steps(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"teams, inboxes and channels", r.places},
		{"tags, saved replies and attributes", r.vocabulary},
		{"help center", r.helpCenter},
		{"AI agent, actions and procedures", r.agent},
		{"AI agent fleet", r.fleet},
		{"companies and people", r.people},
		{"conversations", r.conversations},
		{"past AI runs", r.aiRuns},
		{"workflows", r.workflows},
		{"changelog and feedback", r.community},
	}
	clock := r.s.App.Clock
	at := clock.Now()
	if err := r.workspace(ctx); err != nil {
		return fmt.Errorf("seed workspace and team: %w", err)
	}
	r.s.Log("seeded workspace and team in %s", clock.Now().Sub(at).Round(time.Millisecond))
	for _, st := range steps {
		at := clock.Now()
		err := r.s.App.DB.Transaction(r.scoped(ctx), func(ctx context.Context, _ *database.Tx) error {
			return st.fn(ctx)
		})
		if err != nil {
			return fmt.Errorf("seed %s: %w", st.name, err)
		}
		r.s.Log("seeded %s in %s", st.name, clock.Now().Sub(at).Round(time.Millisecond))
	}
	return nil
}

/*
Find returns the live Stowline workspace, if there is one.
*/
func Find(ctx context.Context, a *app.App) (models.Workspace, bool, error) {
	var id ids.UUID
	err := a.DB.QueryRow(ctx, `SELECT id FROM workspaces WHERE settings ->> 'seed' = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, Slug).Scan(&id)
	if errors.Is(err, database.ErrNotFound) {
		return models.Workspace{}, false, nil
	}
	if err != nil {
		return models.Workspace{}, false, fmt.Errorf("find stowline workspace: %w", err)
	}
	w, err := models.New(a.DB).FindWorkspace(tenancy.With(ctx, id.String()), id)
	if err != nil {
		return models.Workspace{}, false, fmt.Errorf("load stowline workspace: %w", err)
	}
	return w, true, nil
}

/*
clear refuses an existing Stowline workspace, or purges it when asked
to reset. Only the workspace the seed marked is ever touched.
*/
func (s *Seeder) clear(ctx context.Context) error {
	w, found, err := Find(ctx, s.App)
	if err != nil || !found {
		return err
	}
	if !s.Reset {
		return fmt.Errorf("%w (/w/%s)", ErrExists, w.Slug)
	}
	s.Log("purging the earlier Stowline workspace /w/%s", w.Slug)
	due := s.App.Clock.Now().Add(-time.Minute)
	if _, err := s.App.DB.Exec(ctx, `UPDATE workspaces SET deleted_at = $3, purge_after = $3,
			slug = left('stowline-old-' || extract(epoch FROM now())::bigint, 40)
		WHERE id = $1 AND settings ->> 'seed' = $2`, w.ID, Slug, due); err != nil {
		return fmt.Errorf("trash earlier workspace: %w", err)
	}
	// The purge deletes some parents before the rows that point at them,
	// which their foreign keys refuse, so those rows go first.
	err = s.App.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		for _, t := range []string{
			"feedback_posts", "ai_action_calls", "ai_actions", "workflow_runs", "series_steps",
			"series_enrollments",
		} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE workspace_id = $1", w.ID); err != nil {
				return fmt.Errorf("clear %s before purge: %w", t, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	disk, err := s.App.Storage.Default()
	if err != nil {
		return fmt.Errorf("workspace disk: %w", err)
	}
	purger := &jobs.PurgeWorkspace{
		DB: s.App.DB, Clock: s.App.Clock, Disk: disk, Storage: s.App.Storage, Retention: s.App.Retention,
		Log: s.App.Log, Revoke: s.App.Actions.Integrations.RevokeWorkspace.Handle,
	}
	purged, err := purger.Purge(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("purge earlier workspace: %w", err)
	}
	if !purged {
		return errors.New("the earlier Stowline workspace is held by a retention rule; purge it by hand")
	}
	return nil
}

/*
scoped is the context working inside the workspace.
*/
func (r *run) scoped(ctx context.Context) context.Context {
	return tenancy.With(ctx, r.ws.ID.String())
}

/*
user finds the account with the email or creates it with the password.
*/
func (r *run) user(ctx context.Context, name, email, hash string) (models.User, error) {
	u, err := models.New(r.s.App.DB).FindUserByEmail(ctx, strings.ToLower(email))
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return u, fmt.Errorf("find user %s: %w", email, err)
	}
	return r.s.App.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
		Name: name, Email: email, PasswordHash: &hash, Verified: true, CreatedVia: appauth.CreatedViaRegister,
	})
}

/*
workspace creates the workspace through the real action, marks it as
the seed's, takes it past onboarding, keeps its knowledge on keyword
search and adds the team.
*/
func (r *run) workspace(ctx context.Context) error {
	a := r.s.App
	hash, err := a.Hash.Make(ctx, r.s.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	mates := r.plan.Team
	ownerEmail := mates[0].Email
	if r.s.OwnerEmail != "" {
		ownerEmail = r.s.OwnerEmail
	}
	owner, err := r.user(ctx, mates[0].Name, ownerEmail, hash)
	if err != nil {
		return err
	}
	created, err := a.Actions.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
		Owner: owner, Name: Name, JoinedVia: "cli",
	})
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	r.ws = created.Workspace
	r.ownIDs()
	return a.DB.Transaction(r.scoped(ctx), func(ctx context.Context, _ *database.Tx) error {
		return r.staff(ctx, created.Owner.ID, hash)
	})
}

/*
staff marks the workspace as the seed's, takes it past onboarding, keeps
its knowledge on keyword search and adds the teammates.
*/
func (r *run) staff(ctx context.Context, ownerID ids.UUID, hash string) error {
	a := r.s.App
	mates := r.plan.Team
	var err error

	if _, err := a.DB.Exec(ctx, `UPDATE workspaces SET settings = settings || jsonb_build_object('seed', $2::text),
			timezone = 'America/New_York', onboarding_step = 'done', onboarding_completed_at = now(),
			created_at = now() - interval '3 years'
		WHERE id = $1`, r.ws.ID, Slug); err != nil {
		return fmt.Errorf("mark workspace: %w", err)
	}
	if err := r.keywordSearchOnly(ctx); err != nil {
		return err
	}
	if r.ws, err = models.New(a.DB).FindWorkspace(ctx, r.ws.ID); err != nil {
		return fmt.Errorf("reload workspace: %w", err)
	}

	member, err := models.New(a.DB).FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: r.ws.ID, ID: ownerID,
	})
	if err != nil {
		return fmt.Errorf("find owner: %w", err)
	}
	r.owner = authz.NewMembership(r.ws, member, appauth.RolePermissions(enums.RoleOwner))
	r.team = []authz.Membership{r.owner}

	for _, m := range mates[1:] {
		u, err := r.user(ctx, m.Name, m.Email, hash)
		if err != nil {
			return err
		}
		role := enums.Role(m.Role)
		member, err := models.New(a.DB).CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
			WorkspaceID: r.ws.ID, UserID: u.ID, Role: string(role), JoinedVia: new(appauth.MethodInvitation),
			Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("add teammate: %w", err)
		}
		member, err = models.New(a.DB).FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: r.ws.ID, ID: member.ID,
		})
		if err != nil {
			return fmt.Errorf("find teammate: %w", err)
		}
		r.team = append(r.team, authz.NewMembership(r.ws, member, appauth.RolePermissions(role)))
	}
	return nil
}

/*
ownIDs makes the plan's ids the workspace's own: the same plan in two
workspaces, such as a new seed beside one waiting in the trash, never
shares an id.
*/
func (r *run) ownIDs() {
	own := func(id ids.UUID) ids.UUID {
		for i := 8; i < 16; i++ {
			id[i] ^= r.ws.ID[i]
		}
		id[8] = 0x80 | id[8]&0x3f
		return id
	}
	for i := range r.plan.Companies {
		r.plan.Companies[i].ID = own(r.plan.Companies[i].ID)
	}
	for i := range r.plan.Contacts {
		r.plan.Contacts[i].ID = own(r.plan.Contacts[i].ID)
	}
	for i := range r.plan.Conversations {
		r.plan.Conversations[i].ID = own(r.plan.Conversations[i].ID)
	}
}

/*
keywordSearchOnly leaves the workspace without an embedding model, so
publishing the help center queues no paid embedding calls. Creating a
workspace binds the built-in embedder when the install has one and
queues a build; the build is dropped before it can run.
*/
func (r *run) keywordSearchOnly(ctx context.Context) error {
	return r.s.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE knowledge_settings SET active_model_key = NULL, embedding_key_id = NULL,
				pending_model_key = NULL, pending_key_id = NULL, embeddings_status = 'none', reembed_total = 0,
				reembed_done = 0, reembed_started_at = NULL
			WHERE workspace_id = $1`, r.ws.ID); err != nil {
			return fmt.Errorf("unbind embedding model: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM river_job WHERE state IN ('available', 'scheduled', 'retryable')
				AND kind IN ('reembed_workspace_knowledge', 'embed_knowledge_chunks')
				AND args ->> 'workspace_id' = $1::text`, r.ws.ID); err != nil {
			return fmt.Errorf("drop embedding build: %w", err)
		}
		return nil
	})
}

/*
counts reads back how many rows each table holds for the workspace.
*/
func (r *run) counts(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	err := r.s.App.DB.Transaction(r.scoped(ctx), func(ctx context.Context, tx *database.Tx) error {
		for _, t := range CountedTables() {
			var n int64
			err := tx.QueryRow(ctx, "SELECT count(*) FROM "+t+" WHERE workspace_id = $1", r.ws.ID).Scan(&n)
			if err != nil {
				return fmt.Errorf("count %s: %w", t, err)
			}
			out[t] = n
		}
		return nil
	})
	return out, err
}

/*
CountedTables are the tables a run reports row counts for.
*/
func CountedTables() []string {
	return []string{
		"workspace_members", "teams", "inboxes", "channel_connections", "tags", "saved_replies", "collections",
		"articles", "knowledge_chunks", "companies", "contacts", "conversations", "messages", "conversation_tag",
		"conversation_ratings", "ai_runs", "ai_run_steps", "ai_actions", "ai_action_calls", "procedures",
		"procedure_versions", "procedure_test_cases", "workflows",
		"changelog_entries", "feedback_posts", "conversation_events",
	}
}
