/*
|--------------------------------------------------------------------------
| Sources
|--------------------------------------------------------------------------
|
| Admins add, change and remove sources; anyone who reads settings sees
| them. A new source's settings are checked against the real service
| before it is saved, so a wrong key fails at once instead of on the
| first sync. What a source may do comes from its driver: the modes it
| supports, what it can feed and how often it may sync. An import, and
| a source feeding the help center, needs the help center to write to.
|
*/

package knowledgesources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SourceInput is a source to create or a change to one. Settings holds
the driver's settings, secrets included; a blank secret on a change
keeps the stored one. SyncNow starts the first run with the create.
*/
type SourceInput struct {
	Membership   authz.Membership
	ID           ids.UUID
	Type         string
	Name         string
	Mode         string
	Targets      []string
	HelpCenterID *ids.UUID
	Settings     map[string]any
	Include      []string
	Exclude      []string
	Schedule     string
	SyncNow      bool
}

/*
SourceView is a source with what its page shows: its documents counted
by status, its latest run and which secrets are set.
*/
type SourceView struct {
	Source  models.KnowledgeSource
	Counts  map[string]int32
	LastRun *models.KnowledgeSyncRun
	Secrets []string
}

/*
Gallery lists the drivers a source can be added from.
*/
func (e *Engine) Gallery(m authz.Membership) ([]sources.Description, error) {
	if !m.Can("settings:read") {
		return nil, exceptions.ErrForbidden
	}

	return e.Drivers.Descriptions(), nil
}

/*
List returns the workspace's sources.
*/
func (e *Engine) List(ctx context.Context, m authz.Membership) ([]SourceView, error) {
	if !m.Can("settings:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []SourceView

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		rows, err := q.ListKnowledgeSources(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list knowledge sources: %w", err)
		}

		for _, s := range rows {
			view, err := e.view(ctx, q, s)
			if err != nil {
				return err
			}

			out = append(out, view)
		}

		return nil
	})

	return out, err
}

/*
Show returns one source.
*/
func (e *Engine) Show(ctx context.Context, m authz.Membership, id ids.UUID) (SourceView, error) {
	if !m.Can("settings:read") {
		return SourceView{}, exceptions.ErrForbidden
	}

	var out SourceView

	err := e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		s, err := findSource(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		out, err = e.view(ctx, q, s)

		return err
	})

	return out, err
}

/*
view gathers what a source's page shows.
*/
func (e *Engine) view(ctx context.Context, q *models.Queries, s models.KnowledgeSource) (SourceView, error) {
	out := SourceView{Source: s, Counts: map[string]int32{}}

	counts, err := q.CountKnowledgeDocumentsByStatus(ctx, models.CountKnowledgeDocumentsByStatusParams{
		WorkspaceID: s.WorkspaceID, SourceID: s.ID,
	})
	if err != nil {
		return out, fmt.Errorf("count source documents: %w", err)
	}

	for _, c := range counts {
		out.Counts[c.Status] = c.Total
	}

	runs, err := q.ListKnowledgeSyncRuns(ctx, models.ListKnowledgeSyncRunsParams{
		WorkspaceID: s.WorkspaceID, SourceID: s.ID, RowLimit: 1,
	})
	if err != nil {
		return out, fmt.Errorf("find latest run: %w", err)
	}

	if len(runs) > 0 {
		out.LastRun = &runs[0]
	}

	if d, ok := e.Drivers.Driver(s.Type); ok && len(s.CredentialsEncrypted) > 0 {
		out.Secrets = SecretFields(d.Describe())
	}

	return out, nil
}

/*
Create checks the settings against the service and adds the source,
starting its first sync when asked.
*/
func (e *Engine) Create(ctx context.Context, in SourceInput) (SourceView, error) {
	m := in.Membership
	if !m.Can("knowledge:manage") {
		return SourceView{}, exceptions.ErrForbidden
	}

	d, err := e.driverOf(in.Type)
	if err != nil {
		return SourceView{}, err
	}

	if err := validate(d.Describe(), &in); err != nil {
		return SourceView{}, err
	}

	plain, secrets := splitSecrets(d.Describe(), in.Settings)
	if err := checkUpload(m.Workspace.ID, plain); err != nil {
		return SourceView{}, err
	}

	if err := e.check(ctx, m.Workspace.ID, d, configOf(plain, secrets)); err != nil {
		return SourceView{}, err
	}

	return e.insert(ctx, in, plain, secrets)
}

/*
insert stores a checked source, starting its first sync when asked.
*/
func (e *Engine) insert(ctx context.Context, in SourceInput, plain map[string]any, secrets map[string]string,
) (SourceView, error) {
	m := in.Membership
	ws, now, id := m.Workspace.ID, e.Clock.Now(), ids.NewV7At(e.Clock.Now())

	var out SourceView

	err := e.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		sealed, err := e.sealSecrets(ctx, ws, id, secrets)
		if err != nil {
			return err
		}

		config, _ := json.Marshal(plain)

		s, err := q.CreateKnowledgeSource(ctx, models.CreateKnowledgeSourceParams{
			ID: id, WorkspaceID: ws, Type: in.Type, Name: in.Name, Mode: in.Mode, Targets: in.Targets,
			HelpCenterID: in.HelpCenterID, Config: config, Include: clean(in.Include), Exclude: clean(in.Exclude),
			CredentialsEncrypted: sealed, Schedule: in.Schedule, NextSyncAt: nextSync(in.Schedule, now),
			CreatedByMemberID: memberOf(m), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create knowledge source: %w", err)
		}

		if in.SyncNow {
			if _, err := e.start(ctx, tx, s, "create"); err != nil {
				return err
			}
		}

		s, _ = findSource(ctx, q, ws, id)
		out, err = e.view(ctx, q, s)

		return err
	})

	return out, err
}

/*
Update changes a source's name, settings, filters, targets or schedule.
*/
func (e *Engine) Update(ctx context.Context, in SourceInput) (SourceView, error) {
	m := in.Membership
	if !m.Can("knowledge:manage") {
		return SourceView{}, exceptions.ErrForbidden
	}

	ws := m.Workspace.ID

	var out SourceView

	err := e.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		current, err := findSource(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}

		out, err = e.change(ctx, tx, current, in)

		return err
	})

	return out, err
}

/*
change writes an update over the current source.
*/
func (e *Engine) change(ctx context.Context, tx *database.Tx, current models.KnowledgeSource, in SourceInput,
) (SourceView, error) {
	plain, sealed, err := e.changedSettings(ctx, current, &in)
	if err != nil {
		return SourceView{}, err
	}

	q := models.New(tx)
	config, _ := json.Marshal(plain)
	now := e.Clock.Now()

	s, err := q.UpdateKnowledgeSource(ctx, models.UpdateKnowledgeSourceParams{
		Name: in.Name, Targets: in.Targets, HelpCenterID: in.HelpCenterID, Config: config, Include: clean(in.Include),
		Exclude: clean(in.Exclude), Schedule: in.Schedule, NextSyncAt: nextSync(in.Schedule, now),
		CredentialsEncrypted: sealed, Now: now, WorkspaceID: current.WorkspaceID, ID: current.ID,
	})
	if err != nil {
		return SourceView{}, fmt.Errorf("update knowledge source: %w", err)
	}

	if readsDifferently(current, config, clean(in.Include), clean(in.Exclude)) {
		if _, err := e.start(ctx, tx, s, "manual"); err != nil {
			return SourceView{}, err
		}
	}

	return e.view(ctx, q, s)
}

/*
changedSettings validates a change against the source's driver, keeps
the secrets the change left out, connects with the result and seals
the secrets again. It returns the plain settings and the sealed
secrets.
*/
func (e *Engine) changedSettings(ctx context.Context, current models.KnowledgeSource, in *SourceInput,
) (map[string]any, []byte, error) {
	d, err := e.driverOf(current.Type)
	if err != nil {
		return nil, nil, err
	}

	in.Type, in.Mode = current.Type, current.Mode
	fillUnchanged(current, in)

	if err := validate(d.Describe(), in); err != nil {
		return nil, nil, err
	}

	plain, secrets := splitSecrets(d.Describe(), in.Settings)
	if err := checkUpload(current.WorkspaceID, plain); err != nil {
		return nil, nil, err
	}

	if err := e.keepSecrets(ctx, current, secrets); err != nil {
		return nil, nil, err
	}

	if err := e.check(ctx, current.WorkspaceID, d, configOf(plain, secrets)); err != nil {
		return nil, nil, err
	}

	sealed, err := e.sealSecrets(ctx, current.WorkspaceID, current.ID, secrets)

	return plain, sealed, err
}

/*
keepSecrets fills the secrets a change left blank with the stored ones.
*/
func (e *Engine) keepSecrets(ctx context.Context, current models.KnowledgeSource, secrets map[string]string) error {
	kept, err := e.openSecrets(ctx, current)
	if err != nil {
		return err
	}

	for k, v := range kept {
		if _, given := secrets[k]; !given {
			secrets[k] = v
		}
	}

	return nil
}

/*
fillUnchanged keeps what a change leaves out.
*/
func fillUnchanged(current models.KnowledgeSource, in *SourceInput) {
	if in.Name == "" {
		in.Name = current.Name
	}

	if in.Targets == nil {
		in.Targets = current.Targets
	}

	if in.HelpCenterID == nil {
		in.HelpCenterID = current.HelpCenterID
	}

	if in.Settings == nil {
		in.Settings = map[string]any{}
		_ = json.Unmarshal(current.Config, &in.Settings)
	}

	if in.Include == nil {
		in.Include = current.Include
	}

	if in.Exclude == nil {
		in.Exclude = current.Exclude
	}

	if in.Schedule == "" {
		in.Schedule = current.Schedule
	}
}

/*
Delete removes a source with its documents, chunks and runs.
*/
func (e *Engine) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("knowledge:manage") {
		return exceptions.ErrForbidden
	}

	return e.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		n, err := q.DeleteKnowledgeSource(ctx, models.DeleteKnowledgeSourceParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return fmt.Errorf("delete knowledge source: %w", err)
		}

		if n == 0 {
			return ErrSourceNotFound
		}

		return nil
	})
}

/*
validate checks a source's mode, targets, schedule and help center
against what its driver supports, filling the defaults.
*/
func validate(d sources.Description, in *SourceInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 80 {
		return fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").OnField("name")
	}

	if in.Mode == "" {
		in.Mode = string(d.Modes[0])
	}

	if !d.Supports(sources.Mode(in.Mode)) {
		return ErrModeUnsupported
	}

	if len(in.Targets) == 0 {
		in.Targets = defaultTargets(sources.Mode(in.Mode))
	}

	for _, t := range in.Targets {
		if !d.Feeds(sources.Target(t)) {
			return ErrTargetUnsupported
		}
	}

	if (in.Mode == string(sources.ModeImport) || slices.Contains(in.Targets, string(sources.TargetHelpCenter))) &&
		in.HelpCenterID == nil {
		return ErrHelpCenterRequired
	}

	if in.Schedule == "" {
		in.Schedule = d.Schedules[0]
	}

	if in.Mode == string(sources.ModeImport) {
		in.Schedule = "manual"
	}

	if !slices.Contains(d.Schedules, in.Schedule) && in.Schedule != "manual" {
		return ErrScheduleUnsupported
	}

	return nil
}

/*
defaultTargets is what a source feeds when the member does not say: an
import feeds the help center, a sync the agent and the Operator.
*/
func defaultTargets(mode sources.Mode) []string {
	if mode == sources.ModeImport {
		return []string{string(sources.TargetHelpCenter)}
	}

	return []string{string(sources.TargetAgent), string(sources.TargetOperator)}
}

/*
check connects with the settings, turning the driver's refusal into an
answer the member can act on.
*/
func (e *Engine) check(ctx context.Context, ws ids.UUID, d sources.Driver, cfg sources.Config) error {
	cfg, err := e.connected(ctx, ws, d, cfg)
	if err != nil {
		return checkError(err)
	}

	_, err = d.Check(ctx, cfg)

	return checkError(err)
}

/*
connected adds a connected service's token to settings not yet saved.
*/
func (e *Engine) connected(ctx context.Context, ws ids.UUID, d sources.Driver, cfg sources.Config,
) (sources.Config, error) {
	merged := map[string]any{}
	if err := json.Unmarshal(cfg.JSON, &merged); err != nil {
		return cfg, err
	}

	if err := e.connect(ctx, ws, d, merged); err != nil {
		return cfg, err
	}

	cfg.JSON, _ = json.Marshal(merged)

	return cfg, nil
}

/*
checkError turns a driver's refusal into an answer the member can act
on: the setting to fix, or the key the service refused.
*/
func checkError(err error) error {
	if err == nil {
		return nil
	}

	if cfgErr, ok := errors.AsType[*sources.ConfigError](err); ok {
		f := fault.New(fault.Invalid, "knowledge_source_settings_invalid", cfgErr.Message)
		if cfgErr.Field != "" {
			return f.OnField("settings." + cfgErr.Field)
		}

		return f
	}

	if errors.Is(err, sources.ErrUnauthorized) {
		return fault.New(fault.Invalid, "knowledge_source_unauthorized",
			"The service refused these credentials. Check the key and its permissions.")
	}

	return fault.New(fault.Invalid, "knowledge_source_unreachable",
		"The service could not be reached with these settings. "+err.Error())
}

/*
configOf joins plain settings and secrets into a driver's settings.
*/
func configOf(plain map[string]any, secrets map[string]string) sources.Config {
	merged := maps.Clone(plain)
	if merged == nil {
		merged = map[string]any{}
	}

	for k, v := range secrets {
		merged[k] = v
	}

	raw, _ := json.Marshal(merged)

	return sources.Config{JSON: raw}
}

/*
findSource returns a source of the workspace, or 404.
*/
func findSource(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.KnowledgeSource, error) {
	s, err := q.FindKnowledgeSource(ctx, models.FindKnowledgeSourceParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return s, ErrSourceNotFound
	}

	return s, wrap("find knowledge source", err)
}

/*
clean trims patterns and drops empty ones, never nil.
*/
func clean(patterns []string) []string {
	out := []string{}

	for _, p := range patterns {
		if p = strings.TrimSpace(p); p != "" && len(out) < 50 {
			out = append(out, p)
		}
	}

	return out
}

/*
memberOf is the acting member's id, nil for a token without one.
*/
func memberOf(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}

	id := m.Member.ID

	return &id
}
