package reports_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
story builds one conversation's inputs from a start time and minutes.
*/
type story struct {
	t     *testing.T
	start time.Time
	in    reports.Input
}

func teammates() (alice, bob ids.UUID) {
	return ids.Derive(ids.Nil(), "alice"), ids.Derive(ids.Nil(), "bob")
}

func newStory(t *testing.T, start time.Time) *story {
	t.Helper()
	c := models.ReportConversationInputsRow{
		ID: ids.Derive(ids.Nil(), "conversation"), WorkspaceID: ids.Derive(ids.Nil(), "workspace"),
		Number: 1, Channel: "widget", Status: "open", CreatedAt: start,
	}

	s := &story{t: t, start: start, in: reports.Input{Conversation: c, Now: start.Add(30 * 24 * time.Hour)}}
	s.event(0, "created", nil)

	return s
}

func (s *story) at(minutes int) time.Time { return s.start.Add(time.Duration(minutes) * time.Minute) }

func (s *story) customer(minutes int) *story {
	s.in.Messages = append(s.in.Messages, models.ReportMessageInputsRow{AuthorType: "contact", At: s.at(minutes)})

	return s
}

func (s *story) reply(minutes int, member ids.UUID) *story {
	s.in.Messages = append(s.in.Messages, models.ReportMessageInputsRow{
		AuthorType: "member", AuthorID: &member, Substantive: true, At: s.at(minutes),
	})

	return s
}

func (s *story) ai() *story {
	s.in.Messages = append(s.in.Messages, models.ReportMessageInputsRow{
		AuthorType: "agent", Substantive: true, At: s.at(1),
	})

	return s
}

func (s *story) event(minutes int, kind string, member *ids.UUID) *story {
	s.in.Events = append(s.in.Events, models.ReportEventInputsRow{
		Kind: kind, OccurredAt: s.at(minutes), MemberID: member,
	})
	switch kind {
	case "closed":
		s.in.Conversation.Status = "closed"
	case "reopened", "unsnoozed":
		s.in.Conversation.Status = "open"
	case "snoozed":
		s.in.Conversation.Status = "snoozed"
	}

	return s
}

func (s *story) compute() reports.Facts {
	s.t.Helper()
	f := reports.Compute(s.in)
	require.True(s.t, f.Counted)

	return f
}

func secs(minutes int) *int64 {
	v := int64(minutes * 60)

	return &v
}

func monday() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }

func TestComputeResponses(t *testing.T) {
	t.Parallel()

	alice, bob := teammates()

	t.Run("first response skips AI answers and counts each turn", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).ai().reply(10, alice).customer(20).customer(25).reply(40, bob).compute()

		assert.Equal(t, secs(10), f.Row.FirstResponseSeconds)
		assert.Equal(t, secs(1), f.Row.FirstAnswerSeconds, "first answer counts the AI")
		assert.Equal(t, &alice, f.Row.FirstResponderID)
		assert.Equal(t, 2, f.Row.HumanReplies)
		require.Len(t, f.Replies, 2)
		assert.Equal(t, int64(600), f.Replies[0].Seconds)
		assert.Equal(t, int64(20*60), f.Replies[1].Seconds, "a turn starts at the first unanswered message")
		assert.Equal(t, bob, f.Replies[1].MemberID)
	})

	t.Run("an outbound conversation starts at the customer's first message", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).reply(0, alice).customer(60).reply(75, alice).compute()

		assert.Equal(t, monday().Add(time.Hour), f.Row.StartedAt)
		assert.Equal(t, secs(15), f.Row.FirstResponseSeconds)
		require.Len(t, f.Replies, 1)
	})

	t.Run("a close ends a turn nobody answered", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).event(5, "closed", nil).
			customer(100).event(100, "reopened", nil).reply(130, alice).compute()

		require.Len(t, f.Replies, 1)
		assert.Equal(t, monday().Add(100*time.Minute), f.Replies[0].AskedAt)
		assert.Equal(t, int64(30*60), f.Replies[0].Seconds)
	})
}

func TestComputeResolution(t *testing.T) {
	t.Parallel()

	alice, _ := teammates()

	t.Run("a reopen counts the last close and loses first contact resolution", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).reply(5, alice).event(10, "closed", nil).
			customer(60).event(60, "reopened", nil).reply(70, alice).event(90, "closed", nil).compute()

		assert.Equal(t, 2, f.Row.Closes)
		assert.Equal(t, 1, f.Row.Reopens)
		assert.Equal(t, secs(10), f.Row.FirstCloseSeconds)
		assert.Equal(t, secs(90), f.Row.ResolutionSeconds)
		assert.False(t, f.Row.FirstContactResolved)
	})

	t.Run("a reopened conversation still open has no resolution", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).event(10, "closed", nil).event(20, "reopened", nil).compute()

		assert.Nil(t, f.Row.ClosedAt)
		assert.Nil(t, f.Row.ResolutionSeconds)
		assert.Equal(t, secs(10), f.Row.FirstCloseSeconds)
	})

	t.Run("one reply and one close is first contact resolution", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).event(1, "assigned", &alice).reply(5, alice).
			event(10, "closed", nil).compute()

		assert.True(t, f.Row.FirstContactResolved)
		assert.Equal(t, &alice, f.Row.ClosedByMemberID)
		require.Len(t, f.Members, 1)
		assert.Equal(t, int64(9*60), f.Members[0].AssignedOpenSeconds)
	})

	t.Run("snoozed time is not held open time", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).event(0, "assigned", &alice).event(10, "snoozed", nil).
			event(70, "unsnoozed", nil).reply(75, alice).event(80, "closed", nil).compute()

		require.Len(t, f.Members, 1)
		assert.Equal(t, int64(20*60), f.Members[0].AssignedOpenSeconds)
	})
}

func TestComputeImportsAndMerges(t *testing.T) {
	t.Parallel()

	alice, _ := teammates()

	t.Run("a backdated import is dated by its own history", func(t *testing.T) {
		t.Parallel()
		start := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
		s := newStory(t, start).customer(0).reply(45, alice).event(120, "closed", nil)
		s.in.Conversation.Imported = true
		s.in.Now = time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
		f := s.compute()

		assert.True(t, f.Row.Imported)
		assert.Equal(t, start.Add(2*time.Hour), *f.Row.ClosedAt)
		assert.Equal(t, secs(45), f.Row.FirstResponseSeconds)
		assert.Equal(t, secs(120), f.Row.ResolutionSeconds)
	})

	t.Run("a merged-away conversation has no facts", func(t *testing.T) {
		t.Parallel()
		s := newStory(t, monday()).customer(0)
		s.in.Conversation.MergedIntoID = new(ids.New())

		assert.False(t, reports.Compute(s.in).Counted)
	})

	t.Run("spam, test and deleted conversations have no facts", func(t *testing.T) {
		t.Parallel()
		for _, mark := range []func(*models.ReportConversationInputsRow){
			func(c *models.ReportConversationInputsRow) { c.Spam = true },
			func(c *models.ReportConversationInputsRow) { c.IsTest = true },
			func(c *models.ReportConversationInputsRow) { c.DeletedAt = new(monday()) },
		} {
			s := newStory(t, monday()).customer(0)
			mark(&s.in.Conversation)
			assert.False(t, reports.Compute(s.in).Counted)
		}
	})

	t.Run("a conversation with no log is read from its row", func(t *testing.T) {
		t.Parallel()
		s := newStory(t, monday()).customer(0).reply(5, alice)
		s.in.Events = nil
		s.in.Conversation.Status, s.in.Conversation.ClosedAt = "closed", new(monday().Add(time.Hour))
		f := s.compute()

		assert.Equal(t, 1, f.Row.Closes)
		assert.Equal(t, secs(60), f.Row.ResolutionSeconds)
	})
}

func TestComputeAI(t *testing.T) {
	t.Parallel()

	alice, _ := teammates()

	t.Run("a hand-off is logged with its reason and is no AI resolution", func(t *testing.T) {
		t.Parallel()
		s := newStory(t, monday()).customer(0).ai().customer(3)
		s.in.Events = append(s.in.Events, models.ReportEventInputsRow{
			Kind: "ai_handed_off", OccurredAt: s.at(4), Reason: new("asked_for_human"),
		})
		s.reply(20, alice).event(30, "closed", nil)
		s.in.Rating = &models.ReportRatingInputsRow{Score: 5, RatedAt: s.at(40), MemberID: &alice}
		f := s.compute()

		assert.True(t, f.Row.AiInvolved)
		assert.Equal(t, "asked_for_human", *f.Row.AiHandoffReason)
		assert.False(t, f.Row.AiConfirmed)
		assert.Equal(t, &alice, f.Row.RatedMemberID)
		assert.Equal(t, secs(20), f.Row.FirstResponseSeconds)
	})

	t.Run("a helpful AI answer with no teammate is a confirmed resolution credited to dimmi", func(t *testing.T) {
		t.Parallel()
		s := newStory(t, monday()).customer(0).ai().event(10, "closed", nil)
		s.in.Rating = &models.ReportRatingInputsRow{Score: 4, RatedAt: s.at(12)}
		f := s.compute()

		assert.True(t, f.Row.AiConfirmed)
		assert.Nil(t, f.Row.RatedMemberID)
		assert.Nil(t, f.Row.FirstResponseSeconds, "the AI answer is no first response")
		assert.True(t, f.Row.FirstContactResolved)
	})

	t.Run("the AI taking back over keeps the first hand-off", func(t *testing.T) {
		t.Parallel()
		s := newStory(t, monday()).customer(0).ai()
		s.in.Events = append(s.in.Events,
			models.ReportEventInputsRow{Kind: "ai_handed_off", OccurredAt: s.at(2), Reason: new("no_answer")},
			models.ReportEventInputsRow{Kind: "ai_resumed", OccurredAt: s.at(30)},
		)
		f := s.compute()

		assert.Equal(t, s.at(2), *f.Row.AiHandedOffAt)
	})
}

func TestComputeBusinessHours(t *testing.T) {
	t.Parallel()

	alice, _ := teammates()

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	day := [][2]string{{"09:00", "17:00"}}
	weekdays := officehours.Weekly{"mon": day, "tue": day, "wed": day, "thu": day, "fri": day}

	t.Run("a Friday evening message answered on Monday waits one business hour", func(t *testing.T) {
		t.Parallel()
		sched, err := officehours.ParseSchedule("Europe/Berlin", weekdays, nil)
		require.NoError(t, err)

		friday := time.Date(2026, 11, 6, 18, 0, 0, 0, berlin)
		s := newStory(t, friday).customer(0)
		s.reply(int(time.Date(2026, 11, 9, 10, 0, 0, 0, berlin).Sub(friday)/time.Minute), alice)
		s.in.Schedule = sched
		f := s.compute()

		assert.Equal(t, secs(64*60), f.Row.FirstResponseSeconds)
		assert.Equal(t, secs(60), f.Row.FirstResponseBusinessSeconds)
		assert.Equal(t, int64(3600), f.Replies[0].BusinessSeconds)
	})

	t.Run("a holiday Monday counts for nothing", func(t *testing.T) {
		t.Parallel()
		sched, err := officehours.ParseSchedule("Europe/Berlin", weekdays, []officehours.Holiday{{Date: "2026-11-09"}})
		require.NoError(t, err)

		friday := time.Date(2026, 11, 6, 16, 30, 0, 0, berlin)
		s := newStory(t, friday).customer(0)
		s.reply(int(time.Date(2026, 11, 10, 9, 15, 0, 0, berlin).Sub(friday)/time.Minute), alice).
			event(int(time.Date(2026, 11, 10, 9, 30, 0, 0, berlin).Sub(friday)/time.Minute), "closed", nil)
		s.in.Schedule = sched
		f := s.compute()

		assert.Equal(t, secs(45), f.Row.FirstResponseBusinessSeconds)
		assert.Equal(t, secs(60), f.Row.ResolutionBusinessSeconds)
	})

	t.Run("without office hours every second counts", func(t *testing.T) {
		t.Parallel()
		f := newStory(t, monday()).customer(0).reply(600, alice).compute()

		assert.Equal(t, f.Row.FirstResponseSeconds, f.Row.FirstResponseBusinessSeconds)
	})
}
