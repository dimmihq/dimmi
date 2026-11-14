//go:build dev

/*
|--------------------------------------------------------------------------
| WixQA Commands
|--------------------------------------------------------------------------
|
| workspace:wixqa loads the public WixQA help center into a workspace of
| its own; ai:quality asks its questions there and grades the replies
| against the experts' answers with one fixed judge model. The dataset
| is read from where it was downloaded and never copied into the
| repository.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/ai/costsim"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/demo/wixqa"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
subsetOthers is how many articles no question points to a subset keeps.
*/
const subsetOthers = 1500

/*
DefaultJudgeModel is the judge every quality comparison uses: the
largest model with working credit on the install, and not the model
under test, so its grades stay comparable from run to run.
*/
const DefaultJudgeModel = "gpt-6.1-terra"

/*
WorkspaceWixQACommand loads the WixQA help center.
*/
type WorkspaceWixQACommand struct {
	App *app.App
}

/*
Name returns workspace:wixqa.
*/
func (WorkspaceWixQACommand) Name() string { return "workspace:wixqa" }

/*
Description says what the command does.
*/
func (WorkspaceWixQACommand) Description() string {
	return "Load the WixQA help center into a benchmark workspace of its own"
}

/*
Flags declares where the dataset is and whether to load a subset.
*/
func (WorkspaceWixQACommand) Flags(flags *console.Flags) {
	flags.String("dir", wixqa.DefaultDir, "the downloaded dataset folder")
	flags.Bool("subset", "load only the articles the questions point to and 1,500 others")
	flags.Bool("resume", "add the articles an existing WixQA workspace is missing")
}

/*
Handle loads the workspace.
*/
func (c WorkspaceWixQACommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	data, err := dataset(c.App, in.String("dir"))
	if err != nil {
		return err
	}
	s := &wixqa.Seeder{App: c.App, Data: data, Resume: in.Bool("resume"), Log: out.Line}
	if in.Bool("subset") {
		keep, err := subset(ctx, data)
		if err != nil {
			return err
		}
		s.Keep = keep
	}
	res, err := s.Run(ctx)
	if err != nil {
		return err
	}
	out.Info("Loaded %d articles (%d words, %d skipped) into %s (%s).", res.Articles, res.Words, res.Skipped,
		res.Workspace.Name, res.Workspace.Slug)
	return nil
}

/*
subset keeps the articles the questions point to and a fixed set of
others, the first by a hash of their id.
*/
func subset(ctx context.Context, data wixqa.Dataset) (func(wixqa.Doc) bool, error) {
	wanted := map[string]bool{}
	for _, set := range []string{wixqa.SetExpert, wixqa.SetSimulated} {
		qs, err := data.Questions(ctx, set)
		if err != nil {
			return nil, err
		}
		for _, q := range qs {
			for _, id := range q.ArticleIDs {
				wanted[id] = true
			}
		}
	}
	docs, err := data.Corpus(ctx)
	if err != nil {
		return nil, err
	}
	var others []string
	for _, d := range docs {
		if !wanted[d.ID] {
			others = append(others, d.ID)
		}
	}
	slices.SortFunc(others, func(a, b string) int {
		ha, hb := hashing.SHA256Hex([]byte(a)), hashing.SHA256Hex([]byte(b))
		if ha < hb {
			return -1
		}
		return 1
	})
	for _, id := range others[:min(subsetOthers, len(others))] {
		wanted[id] = true
	}
	return func(d wixqa.Doc) bool { return wanted[d.ID] }, nil
}

/*
dataset opens the downloaded dataset's folder.
*/
func dataset(a *app.App, dir string) (wixqa.Dataset, error) {
	disk, err := a.Storage.Build("wixqa-dataset", config.Section{"driver": "local", "root": dir})
	if err != nil {
		return wixqa.Dataset{}, fmt.Errorf("open %s: %w", dir, err)
	}
	return wixqa.Dataset{Disk: disk, Dir: dir}, nil
}

/*
AIQualityCommand grades the agent's answers on WixQA.
*/
type AIQualityCommand struct {
	App *app.App
}

/*
Name returns ai:quality.
*/
func (AIQualityCommand) Name() string { return "ai:quality" }

/*
Description says what the command does.
*/
func (AIQualityCommand) Description() string {
	return "Ask WixQA's questions as customers and grade the replies against the experts' answers"
}

/*
Flags declares the set, the models, the limits and the output.
*/
func (AIQualityCommand) Flags(flags *console.Flags) {
	flags.String("set", wixqa.SetExpert, "the question set: expert or simulated")
	flags.Int("questions", 0, "how many questions to ask, from the start of the set; 0 asks all")
	flags.String("dir", wixqa.DefaultDir, "the downloaded dataset folder")
	flags.String("agent-model", "", "the model the agent answers with; empty keeps the agent's own")
	flags.String("judge-model", DefaultJudgeModel, "the model that grades; keep it fixed across comparisons")
	flags.String("max-spend", "", "required for a real run: the whole run's limit in dollars, judging included")
	flags.String("label", "", "a name for the run's folder")
	flags.Bool("dry-run", "run on an offline model and skip judging: no spend")
	flags.Bool("confirm", "start a real run")
	flags.String("out", "tmp/quality", "the directory reports are written under")
	flags.Int("parallel", 3, "how many questions run at once, all under the one spend limit")
	flags.String("regrade", "", "grade the replies of an earlier run's quality.json again, asking nothing")
}

/*
Handle runs the questions and writes the report.
*/
func (c AIQualityCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	data, err := dataset(c.App, in.String("dir"))
	if err != nil {
		return err
	}
	items, err := qualityItems(ctx, data, in.String("set"), in.Int("questions"))
	if err != nil {
		return err
	}
	o, err := c.options(ctx, in, out)
	if err != nil {
		return err
	}
	budget := costsim.NewBudget(c.App.AI.Prices, o.MaxSpend, o.MaxPerConv)
	sim := AISimulateCommand{App: c.App}
	if _, err := sim.agentModel(ctx, &o); err != nil {
		return err
	}
	out.Line("Questions: %d from the %s set; agent %s; judge %s; limit $%.2f", len(items), in.String("set"),
		o.AgentModel, in.String("judge-model"), float64(o.MaxSpend)/1e6)
	if !o.DryRun && !in.Bool("confirm") {
		out.Warn("Nothing was run. This calls paid models; run again with --confirm to start.")
		return nil
	}
	h := &costsim.Harness{App: c.App, Budget: budget}
	judge, err := c.judge(ctx, o, budget, in.String("judge-model"))
	if err != nil {
		return err
	}
	run := costsim.QualityRun{Set: in.String("set"), Items: items, Judge: judge}
	var report *costsim.QualityReport
	if prior := in.String("regrade"); prior != "" {
		report, err = regrade(ctx, h, prior, run)
	} else {
		report, err = h.RunQuality(ctx, o, run)
	}
	if err != nil {
		return err
	}
	disk, err := c.App.Storage.Build("quality-report", config.Section{"driver": "local", "root": in.String("out")})
	if err != nil {
		return err
	}
	js, md, err := report.Write(ctx, disk, o.Label)
	if err != nil {
		return err
	}
	out.Info("Retrieval hit %.1f%%, average score %.2f, correct %.1f%%, contradicting %d, %.3f¢ per answer, "+
		"%d failed.", report.HitRate*100, report.AvgScore, report.CorrectShare*100, report.Contradicting,
		report.AnswerMicros/1e4, report.Failed)
	out.Line("Report: %s/%s and %s/%s", in.String("out"), js, in.String("out"), md)
	return nil
}

/*
regrade reads an earlier run's report and grades its replies again.
*/
func regrade(ctx context.Context, h *costsim.Harness, path string, run costsim.QualityRun) (
	*costsim.QualityReport, error,
) {
	dir, file := filepath.Split(path)
	disk, err := h.App.Storage.Build("quality-prior", config.Section{"driver": "local", "root": dir})
	if err != nil {
		return nil, err
	}
	raw, err := disk.Get(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var prior costsim.QualityReport
	if err := json.Unmarshal(raw, &prior); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	keys := map[string]costsim.QualityItem{}
	for _, it := range run.Items {
		keys[it.Key] = it
	}
	run.Items = run.Items[:0]
	for _, it := range prior.Items {
		run.Items = append(run.Items, keys[it.Key])
	}
	return h.Regrade(ctx, &prior, run), nil
}

/*
qualityItems reads the questions with the slugs of their articles.
*/
func qualityItems(ctx context.Context, data wixqa.Dataset, set string, n int) ([]costsim.QualityItem, error) {
	qs, err := data.Questions(ctx, set)
	if err != nil {
		return nil, err
	}
	docs, err := data.Corpus(ctx)
	if err != nil {
		return nil, err
	}
	slugs := map[string]string{}
	for _, d := range docs {
		slugs[d.ID] = wixqa.Slug(d)
	}
	if n > 0 && n < len(qs) {
		qs = qs[:n]
	}
	out := make([]costsim.QualityItem, 0, len(qs))
	for i, q := range qs {
		it := costsim.QualityItem{Key: fmt.Sprintf("%s-%03d", set, i+1), Question: q.Question, Expected: q.Answer}
		for _, id := range q.ArticleIDs {
			if s, ok := slugs[id]; ok {
				it.Slugs = append(it.Slugs, s)
			}
		}
		out = append(out, it)
	}
	return out, nil
}

/*
options are the run's options in the WixQA workspace.
*/
func (c AIQualityCommand) options(ctx context.Context, in *console.Input, out *console.Output,
) (costsim.Options, error) {
	dry := in.Bool("dry-run")
	maxSpend, err := dollars(in.String("max-spend"))
	if err != nil {
		return costsim.Options{}, fmt.Errorf("--max-spend: %w", err)
	}
	if !dry && maxSpend <= 0 {
		return costsim.Options{}, errors.New("a real run needs --max-spend in dollars, or use --dry-run")
	}
	owner, inbox, channel, err := wixqaPlace(ctx, c.App)
	if err != nil {
		return costsim.Options{}, err
	}
	label := in.String("label")
	if label == "" {
		label = c.App.Clock.Now().UTC().Format("20060102-150405") + "-" + in.String("set")
	}
	return costsim.Options{
		Owner: owner, Inbox: inbox, Channel: channel, Company: "Wix", Mode: costsim.ModeScripted,
		AgentModel: in.String("agent-model"), MaxSpend: maxSpend, MaxPerConv: maxSpend, DryRun: dry,
		Label: label, Log: out.Line, Parallel: max(in.Int("parallel"), 1),
	}, nil
}

/*
judge builds the judge on the workspace's own connection, spending
under the run's guard.
*/
func (c AIQualityCommand) judge(ctx context.Context, o costsim.Options, budget *costsim.Budget, model string,
) (costsim.Judge, error) {
	if o.DryRun {
		return costsim.Judge{Model: model}, nil
	}
	var key models.AiProviderKey
	err := c.App.DB.Transaction(tenancy.With(ctx, o.Owner.Workspace.ID.String()),
		func(ctx context.Context, tx *database.Tx) error {
			agent, err := models.New(tx).FindAgent(ctx, o.Owner.Workspace.ID)
			if err != nil || agent.AnswerKeyID == nil {
				return errors.Join(err, costsim.ErrNoAgentModel)
			}
			key, err = models.New(tx).FindProviderKey(ctx, models.FindProviderKeyParams{
				WorkspaceID: o.Owner.Workspace.ID, ID: *agent.AnswerKeyID,
			})
			return err
		})
	if err != nil {
		return costsim.Judge{}, fmt.Errorf("judge connection: %w", err)
	}
	llm, err := c.App.LLM.ForKey(ctx, key)
	if err != nil {
		return costsim.Judge{}, err
	}
	return costsim.Judge{LLM: costsim.Guard(llm, budget, key.Provider), Model: model}, nil
}

/*
wixqaPlace finds the WixQA workspace's owner and its messenger.
*/
func wixqaPlace(ctx context.Context, a *app.App) (authz.Membership, ids.UUID, ids.UUID, error) {
	w, found, err := wixqa.Find(ctx, a)
	if err != nil || !found {
		return authz.Membership{}, ids.UUID{}, ids.UUID{}, errors.Join(err,
			errors.New("no WixQA workspace; run workspace:wixqa first"))
	}
	var member models.WorkspaceMember
	var inbox, channel ids.UUID
	err = a.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		var id ids.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM workspace_members WHERE workspace_id = $1 AND role = 'owner'
			AND status = 'active' ORDER BY created_at LIMIT 1`, w.ID).Scan(&id); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT i.id, c.id FROM inboxes i JOIN channel_connections c ON c.inbox_id = i.id
				AND c.type = 'widget' AND c.deleted_at IS NULL
			WHERE i.workspace_id = $1 AND i.deleted_at IS NULL ORDER BY i.created_at LIMIT 1`, w.ID).Scan(&inbox,
			&channel); err != nil {
			return err
		}
		member, err = models.New(tx).FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: w.ID, ID: id,
		})
		return err
	})
	if err != nil {
		return authz.Membership{}, ids.UUID{}, ids.UUID{}, fmt.Errorf("find WixQA owner and messenger: %w", err)
	}
	return authz.NewMembership(w, member, appauth.RolePermissions(enums.RoleOwner)), inbox, channel, nil
}
