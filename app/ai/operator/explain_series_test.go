package operator

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/models"
)

/*
journeyFixture is a journey that sent a welcome email, waited and ended
on a reply.
*/
func journeyFixture(at time.Time) series.JourneyStory {
	left, reason, path := at.Add(time.Hour), "replied", "next"
	conversation := ids.New()
	return series.JourneyStory{
		Enrollment: models.ListPersonSeriesJourneysRow{
			ID: ids.New(), Status: "exited", NodeID: "wait", EnteredAt: at, EnteredBy: "event",
			TriggerRef: new("signed_up"), LeftAt: &left, EndReason: &reason, Subject: series.SubjectContact,
		},
		Version: models.SeriesVersion{ID: ids.New(), Number: 3},
		Spec: series.Spec{
			Nodes: []series.Node{
				{ID: "entry", Kind: series.KindEntry},
				{ID: "hello", Kind: series.KindEmail, Settings: json.RawMessage(`{"subject":"Welcome","body":"Hi"}`)},
				{ID: "wait", Kind: series.KindWait, Settings: json.RawMessage(`{"mode":"duration","amount":2}`)},
			},
			Edges: []series.Edge{
				{From: "entry", Path: "next", To: "hello"}, {From: "hello", Path: "next", To: "wait"},
			},
		},
		Steps: []models.SeriesStep{
			{
				NodeID: "hello", Kind: series.KindEmail, Status: "done", Outcome: new("sent"), PathID: &path,
				EnteredAt: at, Detail: json.RawMessage(`{"sent":1}`),
			},
		},
		Messages: []models.ListSeriesEnrollmentMessagesRow{
			{
				NodeID: "hello", Kind: series.KindEmail, Status: "delivered", Address: "ada@customer.test", SentAt: &at,
				OpenedAt: &at, RepliedAt: &left, ConversationID: &conversation,
			},
		},
	}
}

func TestJourneyText(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	e := &Engine{AppURL: "https://app.test"}

	t.Run("names each step by kind and subject, the path it took, the messages and the end", func(t *testing.T) {
		t.Parallel()
		var b strings.Builder
		e.writeJourney(&b, &session{}, journeyFixture(at))
		text := b.String()

		assert.Contains(t, text, "entered 2026-11-02 09:00:00 UTC by event (signed_up) · follows version 3")
		assert.Contains(t, text, "Ended 2026-11-02 10:00:00 UTC · reason replied · last step wait (wait ")
		assert.Contains(t, text, `1. step hello (email "Welcome") · entered 2026-11-02 09:00:00 UTC · done · `+
			`result sent · took path next → wait · detail {"sent":1}`)
		assert.Contains(t, text, `- from step hello (email "Welcome") to ada@customer.test · delivered · `+
			"sent 2026-11-02 09:00:00 UTC · opened 2026-11-02 09:00:00 UTC · replied 2026-11-02 10:00:00 UTC")
	})

	t.Run("says a person kept out at going live never started", func(t *testing.T) {
		t.Parallel()
		j := journeyFixture(at)
		j.Enrollment.EndReason = new("already_matched")
		var b strings.Builder
		e.writeJourney(&b, &session{}, j)

		assert.Contains(t, b.String(), "Kept out: they already matched when the series went live")
		assert.NotContains(t, b.String(), "Steps reached")
	})

	t.Run("says a held message waits for quiet hours, and until when", func(t *testing.T) {
		t.Parallel()
		until := at.Add(8 * time.Hour)
		var b strings.Builder
		writeWait(&b, models.ListPersonSeriesJourneysRow{
			Wait: json.RawMessage(`{"kind":"cooloff","until":"` + until.Format(time.RFC3339) +
				`","result":"quiet_hours"}`),
		})

		assert.Equal(t, "    Waiting for quiet hours or the daily message cap before the message may go (wait kind "+
			"cooloff) until 2026-11-02 17:00:00 UTC · because quiet_hours\n", b.String())
	})

	t.Run("names a step a newer version dropped", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "gone (no longer in the version this journey follows)", stepName(nil, "gone"))
	})
}
