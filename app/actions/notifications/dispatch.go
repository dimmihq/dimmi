/*
|--------------------------------------------------------------------------
| Dispatch Notifications
|--------------------------------------------------------------------------
|
| An event becomes alerts here, in a job of its own so the request that
| caused it never waits. Who hears about it is decided now: the member
| the event names, else whoever holds the conversation, else the team
| behind it, else everyone, always minus whoever acted and anyone who
| may not see the conversation. Each recipient's bell row is written in
| the job's transaction and goes live once it commits; their email and
| push are queued in the same transaction to go out when the trigger's
| wait is over, if they still matter then.
|
*/

package notifications

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Event is something teammates may be told about: the trigger, where it
happened, who acted, the members it names when it names any, and when.
*/
type Event struct {
	Trigger        string     `json:"trigger"`
	WorkspaceID    ids.UUID   `json:"workspace_id"`
	ConversationID *ids.UUID  `json:"conversation_id,omitempty"`
	MessageID      *ids.UUID  `json:"message_id,omitempty"`
	CallID         *ids.UUID  `json:"call_id,omitempty"`
	TeamID         *ids.UUID  `json:"team_id,omitempty"`
	ActorType      string     `json:"actor_type,omitempty"`
	ActorID        *ids.UUID  `json:"actor_id,omitempty"`
	Targets        []ids.UUID `json:"targets,omitempty"`
	Approval       string     `json:"approval,omitempty"`
	OccurredAt     time.Time  `json:"occurred_at"`
}

/*
Kind names the job that dispatches an event.
*/
func (Event) Kind() string { return "notify_members" }

/*
Options run each event's dispatch with a few retries.
*/
func (Event) Options() queue.Options { return queue.Options{MaxAttempts: 5} }

/*
Dispatcher turns events into alerts.
*/
type Dispatcher struct {
	DB         *database.DB
	Clock      clock.Clock
	Notify     *fnotify.Manager
	Recipients *Recipients
	Checker    *Checker
	Fanout     *appbroadcasting.Fanout
	Links      Links
}

/*
Handle dispatches the job's event.
*/
func (d *Dispatcher) Handle(ctx context.Context, job *queue.Job[Event]) error {
	return d.Dispatch(ctx, job.Args)
}

/*
Dispatch tells everyone who should hear about the event.
*/
func (d *Dispatcher) Dispatch(ctx context.Context, ev Event) error {
	if _, ok := FindTrigger(ev.Trigger); !ok {
		return nil
	}
	scoped := tenancy.With(ctx, ev.WorkspaceID.String())
	return d.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		s, ok, err := readSubject(ctx, q, ev)
		if err != nil || !ok {
			return err
		}
		if s.first && ev.Trigger == TriggerCustomerReplied {
			ev.Trigger = TriggerNewConversation
		}
		recipients, err := d.recipients(ctx, q, ev, &s)
		if err != nil {
			return err
		}
		round := round{tx: tx, ev: ev, subject: &s}
		for _, r := range recipients {
			if err := d.tell(ctx, round, r); err != nil {
				return err
			}
		}
		return nil
	})
}

/*
round is one event being told: its transaction, the event and what it
is about.
*/
type round struct {
	tx      *database.Tx
	ev      Event
	subject *subject
}

/*
tell sends one recipient their alert on the channels they keep on.
*/
func (d *Dispatcher) tell(ctx context.Context, rd round, r Recipient) error {
	tx, ev, s := rd.tx, rd.ev, rd.subject
	t, _ := FindTrigger(ev.Trigger)
	choices, err := LoadChoices(ctx, models.New(tx), ev.WorkspaceID, r.MemberID)
	if err != nil {
		return err
	}
	if s.aiBusy(ev) && !choices.Settings.IncludeAiHandled {
		return nil
	}
	a := s.alert(ev, choices, d.Links)
	a.checker = d.Checker
	if choices.Enabled(t, ChannelInApp) {
		if err := d.Notify.Send(ctx, withChannels(a, "database"), r); err != nil {
			return fmt.Errorf("store notification: %w", err)
		}
		live := withChannels(a, "broadcast")
		tx.AfterCommit(func(ctx context.Context) { _ = d.Notify.Send(ctx, live, r) })
	}
	queued := d.queuedChannels(t, choices, r)
	if len(queued) == 0 {
		return nil
	}
	if err := d.Notify.SendTx(ctx, tx, withChannels(a, queued...), r); err != nil {
		return fmt.Errorf("queue notification: %w", err)
	}
	return nil
}

/*
queuedChannels are the waiting channels that can reach the recipient.
*/
func (d *Dispatcher) queuedChannels(t Trigger, c Choices, r Recipient) []string {
	var out []string
	if c.Enabled(t, ChannelEmail) && r.EmailVerified {
		out = append(out, "mail")
	}
	if c.Enabled(t, ChannelPush) && len(r.Push) > 0 {
		out = append(out, "webpush")
	}
	return out
}

/*
withChannels copies the alert for the channels named.
*/
func withChannels(a *Alert, names ...string) *Alert {
	c := *a
	c.Channels = names
	return &c
}

/*
aiBusy reports whether the agent is answering the conversation, which
quiets news of it for members who did not ask to hear.
*/
func (s *subject) aiBusy(ev Event) bool {
	if s.conversation == nil || s.conversation.AiState != "active" {
		return false
	}
	return ev.Trigger == TriggerNewConversation || ev.Trigger == TriggerCustomerReplied
}

/*
recipients resolves who hears about the event: active members who may
see it, minus whoever acted.
*/
func (d *Dispatcher) recipients(ctx context.Context, q *models.Queries, ev Event, s *subject) ([]Recipient, error) {
	targets, err := d.targets(ctx, q, ev, s)
	if err != nil || targets == nil {
		return nil, err
	}
	all, err := d.Recipients.Load(ctx, q, ev.WorkspaceID, targets)
	if err != nil {
		return nil, err
	}
	restricted, err := d.restricted(ctx, ev, s)
	if err != nil {
		return nil, err
	}
	out := make([]Recipient, 0, len(all))
	for _, r := range all {
		if ev.ActorType == "member" && ev.ActorID != nil && *ev.ActorID == r.MemberID {
			continue
		}
		if !visibleTo(r, restricted, s) || !mayReceive(ev, r) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

/*
targets names the members the event is for: the ones it names, the
holder, the team, or everyone (an empty list). Nil means nobody.
*/
func (d *Dispatcher) targets(ctx context.Context, q *models.Queries, ev Event, s *subject) ([]ids.UUID, error) {
	switch ev.Trigger {
	case TriggerAssigned, TriggerMentioned, TriggerTeamJoined:
		if len(ev.Targets) == 0 {
			return nil, nil
		}
		return ev.Targets, nil
	case TriggerNewConversation:
		return d.audience(ctx, q, s)
	}
	if s.conversation != nil && s.conversation.AssigneeMemberID != nil && ev.Trigger != TriggerApprovalRequired {
		return []ids.UUID{*s.conversation.AssigneeMemberID}, nil
	}
	return d.audience(ctx, q, s)
}

/*
audience is the team behind the conversation, its inbox's team, or
everyone.
*/
func (d *Dispatcher) audience(ctx context.Context, q *models.Queries, s *subject) ([]ids.UUID, error) {
	if s.conversation == nil {
		return []ids.UUID{}, nil
	}
	team := s.conversation.AssigneeTeamID
	if team == nil {
		inbox, err := q.FindInbox(ctx, models.FindInboxParams{
			WorkspaceID: s.conversation.WorkspaceID, ID: s.conversation.InboxID,
		})
		if err != nil {
			return nil, ignoreMissing(err, "load inbox")
		}
		team = inbox.DefaultTeamID
	}
	if team == nil {
		return []ids.UUID{}, nil
	}
	rows, err := q.ListTeamMembers(ctx, models.ListTeamMembersParams{
		WorkspaceID: s.conversation.WorkspaceID, TeamIds: []ids.UUID{*team},
	})
	if err != nil {
		return nil, fmt.Errorf("load team members: %w", err)
	}
	out := make([]ids.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.WorkspaceMember.ID)
	}
	if len(out) == 0 {
		return []ids.UUID{}, nil
	}
	return out, nil
}

/*
restricted returns the members who do not see every conversation, for
events about a conversation.
*/
func (d *Dispatcher) restricted(ctx context.Context, ev Event, s *subject,
) ([]appbroadcasting.RestrictedMember, error) {
	if s.conversation == nil || d.Fanout == nil {
		return nil, nil
	}
	out, err := d.Fanout.Restricted(ctx, ev.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load restricted members: %w", err)
	}
	return out, nil
}

/*
visibleTo reports whether the recipient may see the event's
conversation.
*/
func visibleTo(r Recipient, restricted []appbroadcasting.RestrictedMember, s *subject) bool {
	if s.conversation == nil {
		return true
	}
	i := slices.IndexFunc(restricted, func(m appbroadcasting.RestrictedMember) bool { return m.MemberID == r.MemberID })
	if i < 0 {
		return true
	}
	c := s.conversation
	held := appbroadcasting.Holding{Inbox: c.InboxID, Member: c.AssigneeMemberID, Team: c.AssigneeTeamID}
	return restricted[i].Sees(held)
}

/*
mayReceive reports whether the recipient's role lets them act on the
alert: only those who may decide an action call hear it waits.
*/
func mayReceive(ev Event, r Recipient) bool {
	if ev.Trigger != TriggerApprovalRequired {
		return true
	}
	need := "ai_action_calls:approve"
	if ev.Approval == "admin" {
		need = "ai_actions:manage"
	}
	return slices.Contains(auth.RolePermissions(enums.Role(r.Role)), need)
}
