/*
|--------------------------------------------------------------------------
| Routines
|--------------------------------------------------------------------------
|
| A routine is Operator on a clock. Each run is a real Operator turn on
| a thread of its own, which every teammate who may read what the
| routine reads can open and reply on. The run reads as a narrowed copy
| of the routine's owner: only the permissions the routine was given,
| only the inboxes it names, and only while the owner still holds all of
| it. When the owner left or lost a permission the routine pauses
| instead of running as anyone else.
|
| A routine only ever suggests: nothing it proposes applies on its own,
| whatever the workspace's policies say. When nothing needs attention
| it says so with mark_noop and stays quiet. Its runs spend from the
| month's AI budget like any other Operator run, and from its own
| monthly budget when it has one.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The bounds of a routine's run: how long it may take, how long its
suggestions wait before they count as stale, and how far back a first
run looks.
*/
const (
	routineTimeout = 4 * time.Minute
	suggestionLife = 28 * 24 * time.Hour
	firstWindow    = 7 * 24 * time.Hour
	declinedMemory = 30 * 24 * time.Hour
)

/*
The routine tool groups an admin picks from.
*/
const (
	RoutineGroupConversations = groupConversations
	RoutineGroupKnowledge     = groupKnowledge
	RoutineGroupInsights      = groupInsights
	RoutineGroupChanges       = groupChanges
	RoutineGroupActions       = groupActions
	RoutineGroupGitHub        = "github"
)

/*
RoutineGroups lists every routine tool group.
*/
func RoutineGroups() []string {
	return []string{
		RoutineGroupConversations, RoutineGroupKnowledge, RoutineGroupInsights, RoutineGroupChanges,
		RoutineGroupActions, RoutineGroupGitHub,
	}
}

/*
RoutineScope is what a routine is pointed at beyond its inboxes: the
help centers it drafts in and the GitHub repositories it reads.
*/
type RoutineScope struct {
	HelpCenterIDs []ids.UUID   `json:"help_center_ids,omitempty"`
	GitHub        *GitHubScope `json:"github,omitempty"`
}

/*
GitHubScope is the GitHub connection a routine reads and the only
repositories it may read through it, with the branch merged into, the
labels whose pull requests it skips.
*/
type GitHubScope struct {
	InstallationID ids.UUID `json:"installation_id"`
	Repositories   []string `json:"repositories"`
	BaseBranch     string   `json:"base_branch,omitempty"`
	IgnoreLabels   []string `json:"ignore_labels,omitempty"`
}

/*
RoutineDelivery is who a routine tells about its runs: teammates in the
app, teammates by email, and Slack destinations.
*/
type RoutineDelivery struct {
	InAppMemberIDs      []ids.UUID `json:"in_app_member_ids,omitempty"`
	EmailMemberIDs      []ids.UUID `json:"email_member_ids,omitempty"`
	SlackDestinationIDs []ids.UUID `json:"slack_destination_ids,omitempty"`
}

/*
DeliveryOf reads a routine's stored delivery.
*/
func DeliveryOf(raw json.RawMessage) RoutineDelivery {
	var out RoutineDelivery
	_ = json.Unmarshal(raw, &out)
	return out
}

/*
ScopeOf reads a routine's stored scope.
*/
func ScopeOf(raw json.RawMessage) RoutineScope {
	var out RoutineScope
	_ = json.Unmarshal(raw, &out)
	return out
}

/*
The triggers of a routine's run.
*/
const (
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
	TriggerDryRun    = "dry_run"
)

/*
The outcomes of a routine's run.
*/
const (
	RunFindings = "findings"
	RunNoop     = "noop"
	RunFailed   = "failed"
	RunSkipped  = "skipped"
)

/*
The reasons a routine pauses on its own.
*/
const (
	PauseOwnerLost = "owner_permissions_lost"
	PauseFailures  = "consecutive_failures"
)

/*
routineRun is a routine's run in progress: the routine, its run row, the
scope it reads, whether it is a dry run, and what
it said about its findings.
*/
type routineRun struct {
	row        models.OperatorRoutine
	run        models.OperatorRoutineRun
	scope      RoutineScope
	dryRun     bool
	noop       bool
	noopReason string
}

/*
hasGroup reports whether the routine was given the tool group.
*/
func (r *routineRun) hasGroup(group string) bool {
	return slices.Contains(r.row.ToolGroups, group)
}

/*
allows reports whether the routine may be offered a tool: never drafts
or memories, its own tools always, the GitHub tools when it reads
repositories, and the rest by their group.
*/
func (r *routineRun) allows(e *Engine, t tool) bool {
	switch {
	case t.spec.Name == "remember" || t.spec.Name == "draft_reply":
		return false
	case t.routine || t.spec.Name == "list_recent_changesets":
		return true
	case t.group == RoutineGroupGitHub:
		return r.hasGroup(RoutineGroupGitHub) && r.scope.GitHub != nil && e.Services.GitHub != nil
	}
	return r.hasGroup(routineGroupOf(t))
}

/*
routineGroupOf is the routine tool group of one of Operator's own
tools: searching the knowledge base belongs with the articles it
searches, though the layer offers it beside the conversations.
*/
func routineGroupOf(t tool) string {
	if t.spec.Name == "search_knowledge" {
		return RoutineGroupKnowledge
	}
	return ownGroupOf(t)
}

/*
limits are the model and tool calls the session may make: the
routine's own, or the defaults.
*/
func (s *session) limits() (int, int) {
	if s.routine == nil {
		return maxModelCalls, maxToolCalls
	}
	return int(s.routine.row.MaxModelCalls), int(s.routine.row.MaxToolCalls)
}

/*
FindThread returns a thread the teammate may read: one of their own, or
a routine's thread when they may read what the routine reads.
*/
func FindThread(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID,
) (models.OperatorThread, error) {
	thread, err := q.FindAnyOperatorThread(ctx, models.FindAnyOperatorThreadParams{
		WorkspaceID: m.Workspace.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return thread, exceptions.ErrNotFound
	}
	if err != nil {
		return thread, fmt.Errorf("find operator thread: %w", err)
	}
	switch {
	case thread.MemberID != nil && *thread.MemberID == m.Member.ID:
		return thread, nil
	case thread.MemberID == nil && thread.RoutineID != nil:
		return thread, readableRoutine(ctx, q, m, *thread.RoutineID)
	}
	return thread, exceptions.ErrNotFound
}

/*
readableRoutine refuses a routine's thread or suggestion to a teammate
who may not read what the routine reads.
*/
func readableRoutine(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID) error {
	r, err := q.FindAnyOperatorRoutine(ctx, models.FindAnyOperatorRoutineParams{WorkspaceID: m.Workspace.ID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find operator routine: %w", err)
	}
	if !RoutineReadable(m, r) {
		return exceptions.ErrNotFound
	}
	return nil
}

/*
RoutineReadable reports whether a teammate may read a routine's runs,
threads and suggestions. A routine that names no inbox reads every
conversation, so only teammates who see every conversation read it; one
that names inboxes is read by teammates who see all of them.
*/
func RoutineReadable(m authz.Membership, r models.OperatorRoutine) bool {
	if !m.Can("ai:read") || !m.Can("conversations:read") {
		return false
	}
	scope := m.ConversationScope()
	if !scope.Restricted() {
		return true
	}
	if len(r.InboxIds) == 0 || scope.Mode != enums.ConversationAccessAll {
		return false
	}
	for _, inbox := range r.InboxIds {
		if !scope.OpensInbox(inbox) {
			return false
		}
	}
	return true
}

/*
RoutineVisibility is a teammate's arguments to the queries that list a
routine's suggestions: whether they read routines at all, whether they
see every conversation, and whether their access mode is all.
*/
type RoutineVisibility struct {
	Reads          bool
	SeesEverything bool
	SeesAllModes   bool
}

/*
RoutineVisibilityOf is the teammate's routine visibility.
*/
func RoutineVisibilityOf(m authz.Membership) RoutineVisibility {
	scope := m.ConversationScope()
	return RoutineVisibility{
		Reads:          m.Can("ai:read") && m.Can("conversations:read"),
		SeesEverything: !scope.Restricted(),
		SeesAllModes:   scope.Mode == enums.ConversationAccessAll,
	}
}

/*
LoadMember returns a teammate's membership as it stands now, false when
they left or the workspace is gone.
*/
func LoadMember(ctx context.Context, q *models.Queries, workspaceID, memberID ids.UUID,
) (authz.Membership, bool, error) {
	ws, err := q.FindWorkspace(ctx, workspaceID)
	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("find workspace: %w", err)
	}
	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws.ID, ID: memberID})
	if errors.Is(err, database.ErrNotFound) {
		return authz.Membership{}, false, nil
	}
	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("find member: %w", err)
	}
	if member.Status != string(enums.MemberStatusActive) || ws.DeletedAt != nil {
		return authz.Membership{}, false, nil
	}
	custom, err := q.CustomPermissions(ctx, member)
	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("load custom role: %w", err)
	}
	m := authz.NewMembership(ws, member, authz.HeldPermissions(enums.Role(member.Role), custom))
	if m.AccessTeams, err = authz.LoadAccessTeams(ctx, q, member); err != nil {
		return m, false, err
	}
	if m.HiddenInboxes, err = authz.LoadHiddenInboxes(ctx, q, member); err != nil {
		return m, false, err
	}
	return m, true, nil
}

/*
Narrow returns the membership a routine runs with: the owner's, holding
only the permissions the routine was given and opening only the inboxes
it names. False means the owner no longer holds everything the routine
needs, so it may not run as them.
*/
func Narrow(ctx context.Context, q *models.Queries, owner authz.Membership, r models.OperatorRoutine,
) (authz.Membership, bool, error) {
	if !holds(owner, r) {
		return owner, false, nil
	}
	out := owner
	held := []string{"members:read"}
	for _, p := range r.Permissions {
		if !slices.Contains(held, p) {
			held = append(held, p)
		}
	}
	slices.Sort(held)
	out.Principal = authz.Principal{Role: owner.Principal.Role, Permissions: held}
	if len(r.InboxIds) == 0 {
		return out, true, nil
	}
	inboxes, err := q.ListInboxes(ctx, owner.Workspace.ID)
	if err != nil {
		return out, false, fmt.Errorf("list inboxes: %w", err)
	}
	out.HiddenInboxes = slices.Clone(owner.HiddenInboxes)
	for _, inbox := range inboxes {
		if !slices.Contains(r.InboxIds, inbox.ID) && !slices.Contains(out.HiddenInboxes, inbox.ID) {
			out.HiddenInboxes = append(out.HiddenInboxes, inbox.ID)
		}
	}
	return out, true, nil
}

/*
holds reports whether the owner may manage the AI and holds every
permission and inbox the routine uses.
*/
func holds(owner authz.Membership, r models.OperatorRoutine) bool {
	if !owner.Can("ai_agent:manage") {
		return false
	}
	for _, p := range r.Permissions {
		if !owner.Can(p) {
			return false
		}
	}
	scope := owner.ConversationScope()
	for _, inbox := range r.InboxIds {
		if !scope.OpensInbox(inbox) {
			return false
		}
	}
	return true
}

/*
duplicate checks a suggestion's dedup key: one that still waits, or was
declined lately, is not made again, and the model is told which.
*/
func (e *Engine) duplicate(ctx context.Context, s *session, key string) (string, bool, error) {
	key = dedupKey(key)
	if key == "" {
		return "", false, nil
	}
	var found models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		found, err = models.New(tx).FindOperatorChangesetByDedupKey(ctx, models.FindOperatorChangesetByDedupKeyParams{
			WorkspaceID: s.m.Workspace.ID, DedupKey: &key, DeclinedSince: e.Clock.Now().Add(-declinedMemory),
		})
		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find suggestion by key: %w", err)
	}
	public := models.PublicID(models.PrefixOperatorChangeset, found.ID)
	if found.Status == StatusPending {
		return fmt.Sprintf("Not suggested again: changeset %s with the same dedup_key still waits for review.",
			public), true, nil
	}
	return fmt.Sprintf("Not suggested again: the team declined changeset %s with the same dedup_key on %s.",
		public, found.UpdatedAt.UTC().Format("2006-01-02")), true, nil
}

/*
dedupKey is a dedup key as stored, at most 200 characters.
*/
func dedupKey(key string) string {
	return truncate(strings.TrimSpace(key), 200)
}

/*
ErrRoutineRunning refuses a run while the routine's last one is still
going.
*/
var ErrRoutineRunning = fault.New(fault.Conflict, "routine_running", "This routine is already running.")

/*
routineRules tell the model it runs a routine unattended.
*/
const routineRules = `You are running the routine "{{name}}" on your own: no teammate is watching, and your
answer is the report the team reads later on the routine's thread.
- Do exactly what the routine asks, on the data your tools return, and nothing else.
- Start with routine_get_state to see what earlier runs already covered, and call
  list_recent_changesets before suggesting anything, so you never suggest again what still waits
  or what the team declined. Give every suggestion a stable dedup_key.
- Suggest changes with propose_changes; nothing you suggest applies until a teammate approves it.
- Each propose_changes call changes one article or one procedure (with its tests); suggest changes to
  several in separate calls.
- To edit an article, read it with get_article and send edits: each the exact passage to replace,
  copied word for word, and its replacement. Put what you changed and why in the reason.
- Suggest only the kinds of change the routine asks for; when you cannot make one, say so in your
  report instead of suggesting something else.
- Before you finish, save what the next run needs with routine_set_state.
- When you suggest nothing and the routine asks for no report, you must call mark_noop with a
  one-line reason, then answer in one sentence; otherwise the team is told about a run with
  nothing in it.
- workspace_report reads whole days; a window that starts or ends mid-day is fine as it is.
- Never put customers' names, emails, phone numbers, order ids or companies in an article or
  snippet; cite conversations by number only in a suggestion's reason.
- Write the report in short sections. Each starts with a bold heading alone on its line, then
  a blank line: **Needs you** (what waits for a teammate), **Done** (what you checked and
  found), **Couldn't do** (what you could not suggest, and why). Leave out an empty section.
- In each section write one or two short paragraphs, or up to three short bullets. Name
  articles and procedures by title. Leave the details to the suggestions themselves.`
