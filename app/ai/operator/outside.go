/*
|--------------------------------------------------------------------------
| Agents Outside Operator
|--------------------------------------------------------------------------
|
| An agent a teammate connected does Operator's work without Operator's
| own model: it suggests changes for the team to review, keeps a
| routine's state between its runs, and says when it found nothing. It
| goes through the very tools Operator's runs use, so the same rules
| hold: the workspace's policies decide which changes may be suggested,
| a suggestion made for a routine changes one article or procedure, a
| dry run leaves previews and keeps no state. Nothing such an agent
| suggests ever applies on its own: only a teammate's approval does.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of suggestion an outside agent makes, each through the tool
of Operator's that checks it.
*/
const (
	SuggestArticleEdit   = toolArticleEdit
	SuggestNewArticle    = toolArticleNew
	SuggestProcedureEdit = toolProcedureEdit
	SuggestChanges       = "propose_changes"
)

/*
ErrNoRoutine refuses a routine's tool called without naming a routine.
*/
var ErrNoRoutine = fault.New(fault.Invalid, "routine_required", "Name the routine this is for.").OnField("routine_id")

/*
Outside is who an outside agent acts as: the teammate who connected it,
the routine it does the work of when it names one, and whether this is
a dry run of that routine.
*/
type Outside struct {
	Membership authz.Membership
	Routine    *ids.UUID
	DryRun     bool
}

/*
Suggested is what came of a suggestion: what to tell the agent, and the
changeset when one was made. A suggestion made before makes none.
*/
type Suggested struct {
	Text      string
	Changeset *models.OperatorChangeset
}

/*
outside opens the session an outside agent's call runs in: the
workspace's settings, laid under the routine's when it names one. The
session counts as having read what customers wrote, so nothing it
suggests applies on its own.
*/
func (e *Engine) outside(ctx context.Context, by Outside) (*session, error) {
	s := &session{m: by.Membership, tainted: true, outside: true}
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		if s.settings, err = LoadSettings(ctx, q, s.m.Workspace.ID); err != nil {
			return err
		}
		if !s.settings.Enabled {
			return ErrDisabled
		}
		if by.Routine == nil {
			return nil
		}
		row, err := q.FindOperatorRoutine(ctx, models.FindOperatorRoutineParams{
			WorkspaceID: s.m.Workspace.ID, ID: *by.Routine,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !RoutineReadable(s.m, row)) {
			return exceptions.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find routine: %w", err)
		}
		s.routine = &routineRun{row: row, scope: ScopeOf(row.Scope), dryRun: by.DryRun}
		s.thread.RoutineID = &row.ID
		e.applyRoutineSettings(s)
		return nil
	})
	return s, err
}

/*
Suggest makes a suggestion of the kind from the arguments Operator's
own tool of that kind takes, for a teammate to review.
*/
func (e *Engine) Suggest(ctx context.Context, by Outside, kind string, args json.RawMessage) (Suggested, error) {
	s, err := e.outside(ctx, by)
	if err != nil {
		return Suggested{}, err
	}
	t, ok := e.registry(s)[kind]
	if !ok || len(t.writes) == 0 {
		return Suggested{}, fmt.Errorf("%w: %s", errBadArguments, kind)
	}
	out, err := t.handle(ctx, s, args)
	if err != nil {
		return Suggested{}, taught(err)
	}
	made := Suggested{Text: out.text}
	if len(s.proposed) > 0 {
		made.Changeset = &s.proposed[len(s.proposed)-1]
	}
	return made, nil
}

/*
RecentSuggestions lists the suggestions made in the last days the
teammate may see, newest first, of the status and the routine when
given, without dry run previews.
*/
func (e *Engine) RecentSuggestions(ctx context.Context, by Outside, days int, status string,
) ([]models.OperatorChangeset, error) {
	if days <= 0 {
		days = recentDaysNormal
	}
	since := e.Clock.Now().Add(-time.Duration(min(days, recentDaysMax)) * 24 * time.Hour)
	f := ChangesetFilter{Since: &since, WithoutPreviews: true, Limit: recentListed, Routine: by.Routine}
	if status != "" {
		f.Status = &status
	}
	var rows []models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		rows, err = models.New(tx).ListOperatorChangesets(ctx, ChangesetParams(by.Membership, f))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list changesets: %w", err)
	}
	return rows, nil
}

/*
RoutineState returns what the routine kept from its earlier runs.
*/
func (e *Engine) RoutineState(ctx context.Context, by Outside) (json.RawMessage, error) {
	s, err := e.routineSession(ctx, by)
	if err != nil {
		return nil, err
	}
	out, err := e.routineGetState(ctx, s, nil)
	return json.RawMessage(out.text), err
}

/*
SaveRoutineState merges keys into the routine's state, or replaces it,
and returns the state as it now stands; a dry run saves nothing.
*/
func (e *Engine) SaveRoutineState(ctx context.Context, by Outside, args json.RawMessage) (json.RawMessage, error) {
	s, err := e.routineSession(ctx, by)
	if err != nil {
		return nil, err
	}
	if _, err := e.routineSetState(ctx, s, args); err != nil {
		return nil, taught(err)
	}
	out, err := e.routineGetState(ctx, s, nil)
	return json.RawMessage(out.text), err
}

/*
NothingToChange records that an outside agent ran the routine and found
nothing that needs the team: a completed run that notifies nobody. A
dry run records nothing.
*/
func (e *Engine) NothingToChange(ctx context.Context, by Outside, reason string) error {
	s, err := e.routineSession(ctx, by)
	if err != nil {
		return err
	}
	reason = truncate(strings.TrimSpace(reason), 500)
	if reason == "" {
		return fault.New(fault.Invalid, "reason_required", "Say in one line what you checked and why "+
			"nothing needs changing.").OnField("reason")
	}
	if by.DryRun {
		return nil
	}
	now := e.Clock.Now()
	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		run, err := q.CreateOperatorRoutineRun(ctx, models.CreateOperatorRoutineRunParams{
			WorkspaceID: s.routine.row.WorkspaceID, RoutineID: s.routine.row.ID, Trigger: TriggerManual,
			TriggeredByMemberID: &s.m.Member.ID, Status: "completed", Now: now,
		})
		if err != nil {
			return fmt.Errorf("record routine run: %w", err)
		}
		run.Outcome, run.NoopReason, run.StartedAt, run.CompletedAt = new(RunNoop), &reason, &now, &now
		if err := e.saveRun(ctx, q, run); err != nil {
			return err
		}
		s.routine.run = run
		return e.saveLifecycle(ctx, q, s.routine.row, e.afterRun(s.routine, RunNoop, now))
	})
}

/*
outsidePassages is how many passages an outside agent's search returns.
*/
const outsidePassages = 8

/*
Passage is one passage of the knowledge the AI agent answers from: the
title of what it is from, the article when it is one, its link and its
text.
*/
type Passage struct {
	Title   string
	Article *ids.UUID
	URL     string
	Text    string
}

/*
KnowledgePassages searches the knowledge the AI agent answers customers
from, as Operator's own search does.
*/
func (e *Engine) KnowledgePassages(ctx context.Context, m authz.Membership, query string) ([]Passage, error) {
	if e.Services.Knowledge == nil {
		return nil, nil
	}
	found, err := e.Services.Knowledge.Search(ctx, m.Workspace.ID, query, knowledgeSearch(outsidePassages))
	if err != nil {
		return nil, fmt.Errorf("search knowledge: %w", err)
	}
	out := make([]Passage, len(found.Chunks))
	for i, c := range found.Chunks {
		out[i] = Passage{Title: c.Title, Article: c.ArticleID, URL: c.URL, Text: truncate(c.Text, 1500)}
	}
	return out, nil
}

/*
routineSession opens an outside agent's session on the routine it must
name.
*/
func (e *Engine) routineSession(ctx context.Context, by Outside) (*session, error) {
	if by.Routine == nil {
		return nil, ErrNoRoutine
	}
	return e.outside(ctx, by)
}

/*
Skill is one way of doing a kind of routine work well: its name, when
to read it, and its steps.
*/
type Skill struct {
	Name string
	Use  string
	Body string
}

/*
AllSkills lists the skills, in the order the routine prompt names them.
*/
func AllSkills() []Skill {
	out := make([]Skill, 0, len(skills()))
	for _, s := range skills() {
		out = append(out, Skill{Name: s.name, Use: s.when, Body: skillBody(s.name)})
	}
	return out
}

/*
taught turns a tool's refusal into an error the API answers in the
tool's own words, so an agent reads what to fix.
*/
func taught(err error) error {
	if _, ok := errors.AsType[*fault.Error](err); ok || !errors.Is(err, errBadArguments) {
		return err
	}
	return fault.New(fault.Invalid, "invalid_arguments", strings.TrimPrefix(err.Error(), errBadArguments.Error()+": "))
}
