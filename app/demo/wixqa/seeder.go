/*
|--------------------------------------------------------------------------
| The WixQA Workspace
|--------------------------------------------------------------------------
|
| A second demo workspace whose help center is the WixQA corpus, loaded
| the way a customer imports a help center: each article's HTML becomes
| Markdown and goes through the article actions, so chunking and search
| work on it as they would on a real customer's. The agent answers from
| it on the messenger, where the quality benchmark asks its questions.
| A subset keeps the articles the questions point to and a fixed random
| share of the rest, for when indexing all of them costs too much.
|
*/

package wixqa

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The workspace's names.
*/
const (
	Seed       = "wixqa"
	Name       = "WixQA benchmark"
	CenterName = "WixQA Help Center (Wix.com AI Research)"
	OwnerEmail = "owner@wixqa.test"
	batchSize  = 200
	descLimit  = 240
)

/*
ErrProduction refuses to load the dataset into a production install.
*/
var ErrProduction = errors.New("the WixQA workspace never loads in production")

/*
ErrExists refuses a second WixQA workspace.
*/
var ErrExists = errors.New("a WixQA workspace already exists; trash it first to load again")

/*
Seeder loads the dataset into its own workspace. Keep, when set, limits
the articles to those its function keeps.
*/
type Seeder struct {
	App      *app.App
	Data     Dataset
	Password string
	Keep     func(Doc) bool
	Resume   bool
	Log      func(format string, args ...any)
}

/*
Result is what the load wrote.
*/
type Result struct {
	Workspace models.Workspace
	Owner     authz.Membership
	Articles  int
	Words     int
	Skipped   int
}

/*
Run loads the workspace.
*/
func (s *Seeder) Run(ctx context.Context) (*Result, error) {
	if s.App.Application.Config.App.Env == foundation.EnvProduction {
		return nil, ErrProduction
	}
	if s.Log == nil {
		s.Log = func(string, ...any) {}
	}
	docs, err := s.Data.Corpus(ctx)
	if err != nil {
		return nil, err
	}
	if s.Keep != nil {
		docs = slices.DeleteFunc(docs, func(d Doc) bool { return !s.Keep(d) })
	}
	res := &Result{}
	existing, found, err := Find(ctx, s.App)
	switch {
	case err != nil:
		return nil, err
	case found && !s.Resume:
		return nil, ErrExists
	case found:
		if docs, err = s.resume(ctx, existing, res, docs); err != nil {
			return nil, err
		}
		return res, s.articles(tenancy.With(ctx, existing.ID.String()), res, docs)
	}
	if err := s.workspace(ctx, res); err != nil {
		return nil, err
	}
	scoped := tenancy.With(ctx, res.Workspace.ID.String())
	if err := s.App.DB.Transaction(scoped, func(ctx context.Context, _ *database.Tx) error {
		return s.place(ctx, res.Owner)
	}); err != nil {
		return nil, err
	}
	if err := s.articles(scoped, res, docs); err != nil {
		return nil, err
	}
	return res, nil
}

/*
resume loads the existing workspace's owner and keeps only the articles
it does not hold yet.
*/
func (s *Seeder) resume(ctx context.Context, w models.Workspace, res *Result, docs []Doc) ([]Doc, error) {
	held := map[string]bool{}
	err := s.App.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		rows, err := tx.Query(ctx, `SELECT slug FROM articles WHERE workspace_id = $1 AND deleted_at IS NULL`, w.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var slug string
			if err := rows.Scan(&slug); err != nil {
				rows.Close()
				return err
			}
			held[slug] = true
		}
		rows.Close()
		var id ids.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM workspace_members WHERE workspace_id = $1 AND role = 'owner'
			ORDER BY created_at LIMIT 1`, w.ID).Scan(&id); err != nil {
			return err
		}
		member, err := models.New(tx).FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: w.ID, ID: id,
		})
		res.Workspace, res.Owner = w, authz.NewMembership(w, member, appauth.RolePermissions(enums.RoleOwner))
		return err
	})
	return slices.DeleteFunc(docs, func(d Doc) bool { return held[Slug(d)] }), err
}

/*
Find returns the live WixQA workspace, if there is one.
*/
func Find(ctx context.Context, a *app.App) (models.Workspace, bool, error) {
	var id ids.UUID
	err := a.DB.QueryRow(ctx, `SELECT id FROM workspaces WHERE settings ->> 'seed' = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, Seed).Scan(&id)
	if errors.Is(err, database.ErrNotFound) {
		return models.Workspace{}, false, nil
	}
	if err != nil {
		return models.Workspace{}, false, fmt.Errorf("find wixqa workspace: %w", err)
	}
	w, err := models.New(a.DB).FindWorkspace(tenancy.With(ctx, id.String()), id)
	if err != nil {
		return models.Workspace{}, false, fmt.Errorf("load wixqa workspace: %w", err)
	}
	return w, true, nil
}

/*
workspace creates the workspace and its owner through the real actions
and leaves its knowledge on keyword search until vectors are built on
purpose.
*/
func (s *Seeder) workspace(ctx context.Context, res *Result) error {
	a := s.App
	password := s.Password
	if password == "" {
		password = "wixqa-benchmark"
	}
	hash, err := a.Hash.Make(ctx, password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	owner, err := models.New(a.DB).FindUserByEmail(ctx, OwnerEmail)
	if errors.Is(err, database.ErrNotFound) {
		owner, err = a.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
			Name: "WixQA Owner", Email: OwnerEmail, PasswordHash: &hash, Verified: true,
			CreatedVia: appauth.CreatedViaRegister,
		})
	}
	if err != nil {
		return fmt.Errorf("owner: %w", err)
	}
	created, err := a.Actions.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
		Owner: owner, Name: Name, JoinedVia: "cli",
	})
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	ws := created.Workspace
	return a.DB.Transaction(tenancy.With(ctx, ws.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		res.Workspace = ws
		return s.settle(ctx, tx, res, created.Owner.ID)
	})
}

/*
settle marks the workspace as the benchmark's, takes it past onboarding,
leaves its knowledge on keyword search and loads its owner.
*/
func (s *Seeder) settle(ctx context.Context, tx *database.Tx, res *Result, ownerID ids.UUID) error {
	ws := res.Workspace
	{
		if _, err := tx.Exec(ctx, `UPDATE workspaces SET settings = settings || jsonb_build_object('seed', $2::text),
				onboarding_step = 'done', onboarding_completed_at = now() WHERE id = $1`, ws.ID, Seed); err != nil {
			return fmt.Errorf("mark workspace: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE knowledge_settings SET active_model_key = NULL, embedding_key_id = NULL,
				pending_model_key = NULL, pending_key_id = NULL, embeddings_status = 'none', reembed_total = 0,
				reembed_done = 0, reembed_started_at = NULL WHERE workspace_id = $1`, ws.ID); err != nil {
			return fmt.Errorf("unbind embedding model: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM river_job WHERE state IN ('available', 'scheduled', 'retryable')
				AND kind IN ('reembed_workspace_knowledge', 'embed_knowledge_chunks')
				AND args ->> 'workspace_id' = $1::text`, ws.ID); err != nil {
			return fmt.Errorf("drop embedding build: %w", err)
		}
		q := models.New(tx)
		var err error
		if res.Workspace, err = q.FindWorkspace(ctx, ws.ID); err != nil {
			return err
		}
		member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: ws.ID, ID: ownerID,
		})
		res.Owner = authz.NewMembership(res.Workspace, member, appauth.RolePermissions(enums.RoleOwner))
		return err
	}
}

/*
place connects the default inbox's messenger and API channels and sets
the agent up to answer on the messenger.
*/
func (s *Seeder) place(ctx context.Context, owner authz.Membership) error {
	a := s.App
	inbox, err := models.New(a.DB).FindDefaultInbox(ctx, owner.Workspace.ID)
	if err != nil {
		return fmt.Errorf("find default inbox: %w", err)
	}
	if _, err := a.DB.Exec(ctx, `UPDATE inboxes SET name = 'Support' WHERE workspace_id = $1 AND id = $2`,
		owner.Workspace.ID, inbox.ID); err != nil {
		return err
	}
	for _, kind := range []string{"widget", "api"} {
		if _, err := a.DB.Exec(ctx, `INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status,
				config, last_activity_at)
			SELECT $1, $2, $3, $4, 'active', '{}', now()
			WHERE NOT EXISTS (SELECT 1 FROM channel_connections WHERE workspace_id = $1 AND inbox_id = $2
				AND type = $3 AND deleted_at IS NULL)`, owner.Workspace.ID, inbox.ID, kind, "Wix "+kind); err != nil {
			return fmt.Errorf("connect %s: %w", kind, err)
		}
	}
	view, err := a.Actions.AI.GetAgent.Handle(ctx, ai.GetAgentInput{Membership: owner})
	if err != nil {
		return fmt.Errorf("provision agent: %w", err)
	}
	patch := ai.AgentPatch{
		Name: new("Ava"), Tone: new("friendly"), Schedule: new("always"), DefaultLanguage: new("en"),
		AssignedBehavior: new("skip_when_human_active"), UnknownBehavior: new("offer_handoff"),
		RequireCitations: new(true), Streaming: new(true), EnabledChannels: &[]string{"widget", "api"},
		LinkAllowlist: &[]string{"support.wix.com", "www.wix.com"}, MaxAiReplies: new(int32(12)),
		ProductName: ai.Optional[string]{Set: true, Value: new("Wix")},
	}
	_, err = a.Actions.AI.UpdateAgent.Handle(ctx, ai.UpdateAgentInput{Membership: owner, Patch: patch})
	if err != nil {
		return fmt.Errorf("configure agent: %w", err)
	}
	if view.Agent.AnswerKeyID == nil || view.Agent.AnswerModel == nil {
		s.Log("the install offers no model, so the agent is saved switched off")
		return nil
	}
	_, err = a.Actions.AI.UpdateAgent.Handle(ctx, ai.UpdateAgentInput{
		Membership: owner, Patch: ai.AgentPatch{Enabled: new(true)},
	})
	return err
}

/*
articles imports the corpus into the help center, in batches, each in
its own transaction.
*/
func (s *Seeder) articles(ctx context.Context, res *Result, docs []Doc) error {
	a := s.App
	center, err := models.New(a.DB).FindDefaultHelpCenter(ctx, res.Workspace.ID)
	if err != nil {
		return fmt.Errorf("find help center: %w", err)
	}
	if _, err := a.DB.Exec(ctx, `UPDATE help_centers SET name = $3 WHERE workspace_id = $1 AND id = $2`,
		res.Workspace.ID, center.ID, CenterName); err != nil {
		return err
	}
	for start := 0; start < len(docs); start += batchSize {
		batch := docs[start:min(start+batchSize, len(docs))]
		err := a.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
			for _, d := range batch {
				if err := s.article(ctx, res, center.ID, d); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		s.Log("imported %d of %d articles", min(start+batchSize, len(docs)), len(docs))
	}
	return nil
}

/*
article imports one article from its HTML, as an import from another
help desk would, and publishes it. An article the actions refuse is
skipped and counted.
*/
func (s *Seeder) article(ctx context.Context, res *Result, center ids.UUID, d Doc) error {
	a := s.App
	body, _, err := sources.HTMLToMarkdown(d.HTML)
	if err != nil || strings.TrimSpace(body) == "" {
		body = d.Contents
	}
	title := strings.TrimSpace(d.Title)
	if title == "" {
		title, _, _ = strings.Cut(d.Contents, "\n")
	}
	made, err := a.Actions.Articles.Create(ctx, helpcenters.ArticleInput{
		Membership: res.Owner, HelpCenterID: center, Title: new(clip(title, 200)), Slug: new(Slug(d)),
		Description: new(describe(d.Contents, title)), Body: new(body), Visibility: new("public"),
	})
	if err != nil {
		res.Skipped++
		s.Log("skipped %s: %v", d.ID[:12], err)
		return nil
	}
	if _, err := a.Actions.Articles.Publish(ctx, res.Owner, made.ID); err != nil {
		return fmt.Errorf("publish %s: %w", d.ID[:12], err)
	}
	res.Articles++
	res.Words += len(strings.Fields(body))
	return nil
}

/*
describe is an article's description: the start of its text after the
title.
*/
func describe(contents, title string) string {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(contents), title))
	text = strings.Join(strings.Fields(text), " ")
	return clip(text, descLimit)
}

/*
clip cuts a text to n characters.
*/
func clip(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n])
}
