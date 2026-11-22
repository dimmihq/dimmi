/*
|--------------------------------------------------------------------------
| What An Alert Is About
|--------------------------------------------------------------------------
|
| Before anyone is told, the event is read once into what every alert
| about it shows: the workspace, the conversation and its customer, the
| words that caused it, who acted, and where the link goes. A customer's
| words are cut to a short excerpt and only ever shown to members who
| keep previews on.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/models"
)

/*
excerptLength is the most characters of a customer's words an alert
carries.
*/
const excerptLength = 280

/*
subject is an event read into what its alerts show. Its first field
says the message is the conversation's first from the customer, which
makes a reply a new conversation.
*/
type subject struct {
	workspace    models.Workspace
	conversation *models.Conversation
	customer     string
	excerpt      string
	actor        string
	team         string
	action       string
	first        bool
	routine      string
	outcome      string
}

/*
readSubject loads what the event's alerts show. A conversation or
call that is gone leaves the subject empty, and nobody is told.
*/
func readSubject(ctx context.Context, q *models.Queries, ev Event) (subject, bool, error) {
	ws, err := q.FindWorkspace(ctx, ev.WorkspaceID)
	if err != nil {
		return subject{}, false, fmt.Errorf("load workspace: %w", err)
	}
	s := subject{workspace: ws}
	if ev.ConversationID != nil {
		ok, err := s.readConversation(ctx, q, ev)
		if err != nil || !ok {
			return s, false, err
		}
	}
	if s.actor, err = actorName(ctx, q, ev); err != nil {
		return s, false, err
	}
	if s.team, err = teamName(ctx, q, ev); err != nil {
		return s, false, err
	}
	if ev.CallID != nil {
		call, err := q.FindActionCall(ctx, models.FindActionCallParams{WorkspaceID: ev.WorkspaceID, ID: *ev.CallID})
		if err != nil {
			return s, false, ignoreMissing(err, "load action call")
		}
		s.action = call.Title
	}
	if ev.RoutineRunID != nil {
		if err := s.readRoutineRun(ctx, q, ev); err != nil {
			return s, false, err
		}
	}
	return s, true, nil
}

/*
readRoutineRun loads the routine a run belongs to and how the run went,
as the alert's title and excerpt say.
*/
func (s *subject) readRoutineRun(ctx context.Context, q *models.Queries, ev Event) error {
	run, err := q.FindOperatorRoutineRunByID(ctx, models.FindOperatorRoutineRunByIDParams{
		WorkspaceID: ev.WorkspaceID, ID: *ev.RoutineRunID,
	})
	if err != nil {
		return fmt.Errorf("load routine run: %w", err)
	}
	r, err := q.FindAnyOperatorRoutine(ctx, models.FindAnyOperatorRoutineParams{
		WorkspaceID: ev.WorkspaceID, ID: run.RoutineID,
	})
	if err != nil {
		return fmt.Errorf("load routine: %w", err)
	}
	s.routine, s.outcome = r.Name, routineOutcome(run, r)
	s.excerpt = routineExcerpt(run, r)
	return nil
}

/*
routineOutcome names the title an alert about a run carries: findings,
noop, failed or paused.
*/
func routineOutcome(run models.OperatorRoutineRun, r models.OperatorRoutine) string {
	if r.Status == "paused" && r.PauseReason != nil {
		return "paused"
	}
	if run.Outcome == nil {
		return "noop"
	}
	return *run.Outcome
}

/*
routineExcerpt says in a line what the run left: how many suggestions,
or why it failed or paused.
*/
func routineExcerpt(run models.OperatorRoutineRun, r models.OperatorRoutine) string {
	switch routineOutcome(run, r) {
	case "paused":
		return "Paused: " + strings.ReplaceAll(*r.PauseReason, "_", " ") + "."
	case "failed":
		if run.ErrorMessage != nil {
			return truncate(*run.ErrorMessage, excerptLength)
		}
		return "The run failed."
	case "findings":
		if run.ChangesetsCount == 1 {
			return "1 suggested change is waiting for review."
		}
		return strconv.Itoa(int(run.ChangesetsCount)) + " suggested changes are waiting for review."
	}
	if run.NoopReason != nil {
		return truncate(*run.NoopReason, excerptLength)
	}
	return "Nothing needs attention."
}

/*
readConversation loads the conversation, its customer's name and the
message's words.
*/
func (s *subject) readConversation(ctx context.Context, q *models.Queries, ev Event) (bool, error) {
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: ev.WorkspaceID, ID: *ev.ConversationID,
	})
	if err != nil {
		return false, ignoreMissing(err, "load conversation")
	}
	if c.DeletedAt != nil || c.Spam {
		return false, nil
	}
	s.conversation = &c
	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ev.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return false, ignoreMissing(err, "load customer")
	}
	if contact.BlockedAt != nil {
		return false, nil
	}
	s.customer = contactName(contact)
	if ev.MessageID == nil {
		return true, nil
	}
	m, err := q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: ev.WorkspaceID, ConversationID: c.ID, ID: *ev.MessageID,
	})
	if err == nil && m.Body != nil {
		s.excerpt = truncate(strings.TrimSpace(markdown.PlainText([]byte(*m.Body))), excerptLength)
	}
	if err != nil {
		return true, ignoreMissing(err, "load message")
	}
	first, err := q.FirstCustomerMessageID(ctx, models.FirstCustomerMessageIDParams{
		WorkspaceID: ev.WorkspaceID, ConversationID: c.ID,
	})
	s.first = err == nil && first == m.ID
	return true, ignoreMissing(err, "find first message")
}

/*
contactName is how alerts name a customer: their name, else their
address, else a plain stand-in.
*/
func contactName(c models.Contact) string {
	if c.Name != nil && strings.TrimSpace(*c.Name) != "" {
		return strings.TrimSpace(*c.Name)
	}
	if c.Email != nil && *c.Email != "" {
		return *c.Email
	}
	return ""
}

/*
actorName is the name of the member who acted, empty for anyone else.
*/
func actorName(ctx context.Context, q *models.Queries, ev Event) (string, error) {
	if ev.ActorType != "member" || ev.ActorID == nil {
		return "", nil
	}
	m, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
		WorkspaceID: ev.WorkspaceID, ID: *ev.ActorID,
	})
	if err != nil {
		return "", ignoreMissing(err, "load acting member")
	}
	return m.User.Name, nil
}

/*
teamName is the name of the team the event names, empty for none.
*/
func teamName(ctx context.Context, q *models.Queries, ev Event) (string, error) {
	if ev.TeamID == nil {
		return "", nil
	}
	t, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: ev.WorkspaceID, ID: *ev.TeamID})
	if err != nil {
		return "", ignoreMissing(err, "load team")
	}
	return t.Team.Name, nil
}

/*
ignoreMissing treats a row that is gone as nothing to load.
*/
func ignoreMissing(err error, doing string) error {
	if err == nil || errors.Is(err, database.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("%s: %w", doing, err)
}

/*
alert writes the subject's alert for one recipient.
*/
func (s *subject) alert(ev Event, choices Choices, links Links) *Alert {
	a := &Alert{
		Trigger: ev.Trigger, WorkspaceID: ev.WorkspaceID, Workspace: s.workspace.Name,
		ConversationID: ev.ConversationID, MessageID: ev.MessageID, CallID: ev.CallID, ActorName: s.actor,
		OccurredAt: ev.OccurredAt, ManageLink: links.Settings(s.workspace.Slug),
		Link: links.Home(s.workspace.Slug), TitleKey: ev.Trigger, TitleArgs: s.titleArgs(),
	}
	if s.conversation != nil {
		a.Number = s.conversation.Number
		a.Link = links.Conversation(s.workspace.Slug, s.conversation.ID)
	}
	if choices.Settings.ShowPreviews {
		a.Body = s.excerpt
	}
	if ev.Trigger == TriggerAssigned && s.actor == "" {
		a.TitleKey = "conversation_assigned_system"
	}
	if ev.Trigger == TriggerRoutineFinished {
		a.TitleKey, a.Link = ev.Trigger+"_"+s.outcome, links.Operator(s.workspace.Slug)
		if !choices.Settings.ShowPreviews {
			a.Body = s.excerpt
		}
	}
	return a
}

/*
titleArgs are the words a title template fills in.
*/
func (s *subject) titleArgs() map[string]string {
	name := s.customer
	if name == "" && s.conversation != nil {
		name = "#" + strconv.FormatInt(s.conversation.Number, 10)
	}
	return map[string]string{
		"name": name, "actor": s.actor, "team": s.team, "action": s.action, "routine": s.routine,
	}
}

/*
Links writes absolute dashboard addresses from the install's base URL.
Key signs unsubscribe links; without one, emails carry none.
*/
type Links struct {
	Base string
	Key  []byte
}

/*
Conversation links a conversation in the workspace.
*/
func (l Links) Conversation(slug string, id ids.UUID) string {
	return strings.TrimRight(l.Base, "/") + "/d/" + slug + "/inbox/all/" +
		models.PublicID(models.PrefixConversation, id)
}

/*
Settings links the member's notification settings.
*/
func (l Links) Settings(slug string) string {
	return strings.TrimRight(l.Base, "/") + "/d/" + slug + "/settings/notifications"
}

/*
Operator links Operator's page in the workspace, where routines live.
*/
func (l Links) Operator(slug string) string {
	return strings.TrimRight(l.Base, "/") + "/d/" + slug + "/agent?tab=operator"
}

/*
Home links the workspace's notifications page.
*/
func (l Links) Home(slug string) string {
	return strings.TrimRight(l.Base, "/") + "/d/" + slug + "/notifications"
}
