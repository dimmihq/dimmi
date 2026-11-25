/*
|--------------------------------------------------------------------------
| Publishing Procedures
|--------------------------------------------------------------------------
|
| Publishing makes the working copy what dimmi follows. A procedure is
| published only whole: when it applies, at least one step, every step
| written, and everything a step names existing: each action published,
| each article published, each team, tag, ticket type and saved audience
| still there, and each sub-procedure live, with no loop back to itself
| and no chain deeper than three. Every publish is kept as a numbered
| version, which the procedure then names as live.
|
| Unpublishing takes it out of dimmi's hands at once and keeps the
| working copy. A sub-procedure a live procedure follows can be neither
| unpublished nor deleted until that procedure stops following it.
|
*/

package procedures

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
ErrStillFollowed refuses to take away a sub-procedure live procedures
follow.
*/
var ErrStillFollowed = fault.New(fault.Conflict, "procedure_in_use",
	"Live procedures follow this one. Remove it from them first.")

/*
Publish checks the working copy and makes it live.
*/
func (p *Procedures) Publish(ctx context.Context, m authz.Membership, id ids.UUID) (models.Procedure, error) {
	return p.PublishNote(ctx, m, id, "")
}

/*
PublishNote publishes with a note saying what changed, kept on the
version.
*/
func (p *Procedures) PublishNote(ctx context.Context, m authz.Membership, id ids.UUID, note string,
) (models.Procedure, error) {
	if !m.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if err := p.publishable(ctx, q, current); err != nil {
			return err
		}

		out, err = p.makeLive(ctx, q, publishing{m: m, current: current, note: note})

		return err
	})
	if err == nil {
		p.embed(ctx, out)
	}

	return out, err
}

/*
publishing is a publish: who publishes, the procedure as it stands, and
the note on what changed.
*/
type publishing struct {
	m       authz.Membership
	current models.Procedure
	note    string
}

/*
makeLive publishes the working copy, keeps it as the next version and
records it.
*/
func (p *Procedures) makeLive(ctx context.Context, q *models.Queries, pub publishing) (models.Procedure, error) {
	m, current, note, now := pub.m, pub.current, pub.note, p.Clock.Now()
	out, err := q.PublishProcedure(ctx, models.PublishProcedureParams{
		MemberID: &m.Member.ID, Now: &now, WorkspaceID: m.Workspace.ID, ID: current.ID,
	})
	if err != nil {
		return out, wrap("publish procedure", err)
	}

	var changeNote *string
	if note = strings.TrimSpace(note); note != "" {
		changeNote = &note
	}
	version, err := q.CreateProcedureVersion(ctx, models.CreateProcedureVersionParams{
		WorkspaceID: m.Workspace.ID, ProcedureID: current.ID, Name: current.Name, Definition: current.Draft,
		ChangeNote: changeNote, MemberID: &m.Member.ID, Now: now,
	})
	if err != nil {
		return out, wrap("keep procedure version", err)
	}

	out, err = q.SetProcedureLiveVersion(ctx, models.SetProcedureLiveVersionParams{
		VersionID: &version.ID, WorkspaceID: m.Workspace.ID, ID: current.ID,
	})
	if err != nil {
		return out, wrap("name live version", err)
	}

	return out, p.audit(ctx, m, auditing{
		event: audit.EventProcedurePublished, proc: out, changes: map[string]any{"version": version.Version},
	})
}

/*
embed keeps the vector of a published procedure's description, so a
workspace with many procedures offers dimmi the closest. A workspace
without embeddings, or a provider that fails, leaves it without one.
*/
func (p *Procedures) embed(ctx context.Context, proc models.Procedure) {
	if p.Vectors == nil {
		return
	}

	d, err := procedure.Parse(proc.Live)
	if err != nil {
		return
	}

	vector, model, err := p.Vectors.EmbedDocument(ctx, proc.WorkspaceID, proc.Name+". "+d.Description)
	if err != nil {
		return
	}

	text := vectorText(vector)
	_ = p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).SetProcedureEmbedding(ctx, models.SetProcedureEmbeddingParams{
			Embedding: &text, ModelKey: &model, WorkspaceID: proc.WorkspaceID, ID: proc.ID,
		})
	})
}

/*
publishable lists every problem of the working copy, references
included.
*/
func (p *Procedures) publishable(ctx context.Context, q *models.Queries, current models.Procedure) error {
	d, err := procedure.Parse(current.Draft)
	if err != nil {
		return ErrInvalid.With("reason", err.Error())
	}

	issues := procedure.Check(d)
	more, err := p.referenceIssues(ctx, q, current, d)
	if err != nil {
		return err
	}

	issues = append(issues, more...)
	if len(issues) > 0 {
		return ErrInvalid.With("errors", issues)
	}

	return nil
}

/*
reference is one kind of thing a procedure names: the chip kind and
field an issue points at, what it says, the references, and whether
one is ready.
*/
type reference struct {
	kind, field, message string
	list                 []string
	ready                func(string) (bool, error)
}

/*
referenceIssues lists what the definition names that does not exist or
cannot be followed.
*/
func (p *Procedures) referenceIssues(ctx context.Context, q *models.Queries, current models.Procedure,
	d procedure.Definition,
) ([]procedure.Issue, error) {
	var out []procedure.Issue
	for _, c := range p.references(ctx, q, current.WorkspaceID, d) {
		for _, ref := range c.list {
			ok, err := c.ready(ref)
			if err != nil {
				return nil, err
			}
			if !ok {
				out = append(out, procedure.Issue{
					Code: "reference_missing", Field: c.field, StepID: procedure.StepWith(d, c.kind, ref),
					Message: c.message,
				})
			}
		}
	}

	subs, err := p.subIssues(ctx, q, current, d)

	return append(out, subs...), err
}

/*
references lists what the definition names, by kind, with how to check
each one.
*/
func (p *Procedures) references(ctx context.Context, q *models.Queries, ws ids.UUID, d procedure.Definition,
) []reference {
	refs := procedure.References(d)
	var audiences []string
	if d.Scope != nil {
		audiences = d.Scope.AudienceIDs
	}
	finder := func(prefix models.Prefix, find finder) func(string) (bool, error) {
		return func(r string) (bool, error) { return p.exists(ctx, prefix, r, find) }
	}

	return []reference{
		{
			procedure.BindAction, "action_id", "An action this procedure takes is not published.", refs.Actions,
			func(r string) (bool, error) { return p.actionReady(ctx, q, ws, r) },
		},
		{
			procedure.BindArticle, "bindings", "An article this procedure cites is not published.", refs.Articles,
			func(r string) (bool, error) { return p.articleReady(ctx, q, ws, r) },
		},
		{
			procedure.BindHandoff, "bindings", "A team this procedure hands over to no longer exists.", refs.Teams,
			finder(models.PrefixTeam, teamFinder(q, ws)),
		},
		{
			procedure.BindTag, "bindings", "A tag this procedure adds no longer exists.", refs.Tags,
			finder(models.PrefixTag, tagFinder(q, ws)),
		},
		{
			procedure.BindTicketType, "bindings", "A ticket type this procedure opens no longer exists.",
			refs.TicketTypes, finder(models.PrefixTicketType, ticketTypeFinder(q, ws)),
		},
		{
			"", "scope", "A saved audience this procedure is for no longer exists.", audiences,
			finder(models.PrefixAudience, audienceFinder(q, ws)),
		},
	}
}

/*
subIssues checks every sub-procedure is live, never leads back to the
procedure, and nests at most three deep.
*/
func (p *Procedures) subIssues(ctx context.Context, q *models.Queries, current models.Procedure,
	d procedure.Definition,
) ([]procedure.Issue, error) {
	subs := procedure.References(d).SubProcedures
	if len(subs) == 0 {
		return nil, nil
	}

	rows, err := q.ListLiveProcedures(ctx, current.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list live procedures: %w", err)
	}

	self := models.PublicID(models.PrefixProcedure, current.ID)
	graph := map[string][]string{self: subs}
	for _, r := range rows {
		public := models.PublicID(models.PrefixProcedure, r.ID)
		if public == self {
			continue
		}
		if def, err := procedure.Parse(r.Live); err == nil {
			graph[public] = procedure.References(def).SubProcedures
		}
	}

	var out []procedure.Issue
	for _, sub := range subs {
		if _, live := graph[sub]; !live || sub == self {
			out = append(out, procedure.Issue{
				Code: "reference_missing", Field: "sub_procedure_id",
				StepID:  procedure.StepWith(d, procedure.BindSubProcedure, sub),
				Message: "A sub-procedure this procedure follows is not live.",
			})
		}
	}

	if loop := procedure.Cycle(self, func(id string) []string { return graph[id] }); loop != nil {
		message := "Sub-procedures nest more than three deep."
		if slices.Contains(loop[:len(loop)-1], loop[len(loop)-1]) {
			message = "Sub-procedures lead back to a procedure already being followed: " +
				strings.Join(loop, " → ") + "."
		}
		out = append(out, procedure.Issue{Code: "invalid_setting", Field: "sub_procedure_id", Message: message})
	}

	return out, nil
}

/*
actionReady reports whether the action exists and is published.
*/
func (p *Procedures) actionReady(ctx context.Context, q *models.Queries, ws ids.UUID, public string) (bool, error) {
	id, ok := p.decode(models.PrefixAiAction, public)
	if !ok {
		return false, nil
	}

	action, err := q.FindAction(ctx, models.FindActionParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find action: %w", err)
	}

	return action.Status == "published", nil
}

/*
articleReady reports whether the article exists and is published.
*/
func (p *Procedures) articleReady(ctx context.Context, q *models.Queries, ws ids.UUID, public string) (bool, error) {
	id, ok := p.decode(models.PrefixArticle, public)
	if !ok {
		return false, nil
	}

	_, err := q.FindWorkspaceArticleDocument(ctx, models.FindWorkspaceArticleDocumentParams{
		WorkspaceID: ws, ID: id, Visibilities: []string{"public", "agent_only"},
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	return err == nil, wrap("find article", err)
}

/*
finder looks one row up by id.
*/
type finder func(ctx context.Context, id ids.UUID) error

func teamFinder(q *models.Queries, ws ids.UUID) finder {
	return func(ctx context.Context, id ids.UUID) error {
		_, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: ws, ID: id})
		return err
	}
}

func tagFinder(q *models.Queries, ws ids.UUID) finder {
	return func(ctx context.Context, id ids.UUID) error {
		_, err := q.FindTag(ctx, models.FindTagParams{WorkspaceID: ws, ID: id})
		return err
	}
}

func ticketTypeFinder(q *models.Queries, ws ids.UUID) finder {
	return func(ctx context.Context, id ids.UUID) error {
		_, err := q.FindTicketType(ctx, models.FindTicketTypeParams{WorkspaceID: ws, ID: id})
		return err
	}
}

func audienceFinder(q *models.Queries, ws ids.UUID) finder {
	return func(ctx context.Context, id ids.UUID) error {
		_, err := q.FindAudience(ctx, models.FindAudienceParams{WorkspaceID: ws, ID: id})
		return err
	}
}

/*
exists reports whether a public id of a kind names a row.
*/
func (p *Procedures) exists(ctx context.Context, prefix models.Prefix, public string, find finder) (bool, error) {
	id, ok := p.decode(prefix, public)
	if !ok {
		return false, nil
	}

	err := find(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	return err == nil, wrap("find reference", err)
}

/*
decode reads a public id of a kind, false when it is not one.
*/
func (p *Procedures) decode(prefix models.Prefix, public string) (ids.UUID, bool) {
	id, err := p.IDs.Decode(string(prefix), public)

	return id, err == nil
}

/*
Unpublish takes a procedure out of dimmi's hands.
*/
func (p *Procedures) Unpublish(ctx context.Context, m authz.Membership, id ids.UUID) (models.Procedure, error) {
	if !m.Can("ai_agent:manage") {
		return models.Procedure{}, exceptions.ErrForbidden
	}

	var out models.Procedure
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		if err := followedBy(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		var err error
		out, err = q.UnpublishProcedure(ctx, models.UnpublishProcedureParams{
			MemberID: &m.Member.ID, Now: p.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return wrap("unpublish procedure", err)
		}

		return p.audit(ctx, m, auditing{event: audit.EventProcedureUnpublished, proc: out})
	})

	return out, err
}

/*
followedBy refuses when live procedures follow this one as a
sub-procedure, naming them.
*/
func followedBy(ctx context.Context, q *models.Queries, ws, id ids.UUID) error {
	rows, err := q.ListLiveProcedures(ctx, ws)
	if err != nil {
		return fmt.Errorf("list live procedures: %w", err)
	}

	public := models.PublicID(models.PrefixProcedure, id)
	var parents []map[string]string
	for _, r := range rows {
		if r.ID == id {
			continue
		}
		def, err := procedure.Parse(r.Live)
		if err == nil && slices.Contains(procedure.References(def).SubProcedures, public) {
			parents = append(parents, map[string]string{
				"id": models.PublicID(models.PrefixProcedure, r.ID), "name": r.Name,
			})
		}
	}

	if len(parents) > 0 {
		return ErrStillFollowed.With("parents", parents)
	}

	return nil
}

/*
Delete removes a procedure.
*/
func (p *Procedures) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("ai_agent:manage") {
		return exceptions.ErrForbidden
	}

	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		if err := followedBy(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		return wrap("delete procedure", q.DeleteProcedure(ctx, models.DeleteProcedureParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: new(p.Clock.Now()),
		}))
	})
}

/*
auditing is one change to record: the event, the procedure, and what
changed.
*/
type auditing struct {
	event   string
	proc    models.Procedure
	changes map[string]any
}

/*
audit records a change to a procedure in the workspace's audit log.
*/
func (p *Procedures) audit(ctx context.Context, m authz.Membership, a auditing) error {
	if p.Audit == nil {
		return nil
	}

	_, err := p.Audit.Handle(ctx, audit.Entry{
		WorkspaceID: m.Workspace.ID, Event: a.event, ActorID: m.Member.ID,
		Subject: audit.Subject{Type: "procedure", ID: a.proc.ID, Label: a.proc.Name}, Changes: a.changes,
	})

	return err
}
