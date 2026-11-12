package conversations_test

import (
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
history is a moment of imported history: the given minutes after nine
in the morning of 3 March 2024.
*/
func history(minutes int) time.Time {
	return time.Date(2024, time.March, 3, 9, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

/*
stamp returns the time as a request sends it.
*/
func stamp(minutes int) string {
	return history(minutes).Format(time.RFC3339)
}

/*
timeAt reads the time at the path of the response.
*/
func timeAt(t *testing.T, res *testkit.Response, path string) time.Time {
	t.Helper()

	at, err := time.Parse(time.RFC3339Nano, res.String(path))
	require.NoError(t, err, "no time at %s: %s", path, res.Body)

	return at.UTC()
}

/*
imported starts an imported conversation the customer wrote at the
minute, and returns its public id.
*/
func (d desk) imported(t *testing.T, extra testkit.JSON) string {
	t.Helper()

	body := testkit.JSON{
		"inbox_id": d.inboxID(), "author": "contact", "subject": "Old refund", "body": "Where is my refund?",
		"contact": testkit.JSON{"email": "old@example.com", "name": "Old"}, "created_at": stamp(0),
	}
	maps.Copy(body, extra)

	return d.owner.PostJSON("/api/v1/conversations", body).AssertCreated().String("data.id")
}

func TestImportHistory(t *testing.T) {
	t.Parallel()

	t.Run("replays a conversation at its own times and keeps the inbox columns right", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.imported(t, nil)
		path := "/api/v1/conversations/" + id

		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Hello?", "created_at": stamp(5)}).AssertCreated()
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Checking", "created_at": stamp(8)}).AssertCreated()
		d.owner.PostJSON(path+"/replies", testkit.JSON{
			"body": "Refunded", "created_at": stamp(10), "author_id": d.ownerID(), "then": "close",
		}).AssertCreated()

		res := d.owner.GetJSON(path).AssertOK()
		assert.Equal(t, history(0), timeAt(t, res, "data.created_at"))
		assert.Equal(t, history(0), timeAt(t, res, "data.first_customer_message_at"))
		assert.Equal(t, history(10), timeAt(t, res, "data.first_reply_at"))
		assert.Equal(t, history(10), timeAt(t, res, "data.last_message_at"))
		assert.Equal(t, history(10), timeAt(t, res, "data.closed_at"))
		res.AssertJSONPath("data.status", "closed").AssertJSONPath("data.message_count", 3).
			AssertJSONPath("data.waiting_since", nil).AssertJSONPath("data.assignee", nil)

		timeline := d.owner.GetJSON(path + "/messages").AssertOK()
		var page struct {
			Data []struct {
				Type      string    `json:"type"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"data"`
		}
		timeline.Decode(&page)

		require.NotEmpty(t, page.Data)
		for i := 1; i < len(page.Data); i++ {
			assert.False(t, page.Data[i].CreatedAt.Before(page.Data[i-1].CreatedAt), "timeline out of order")
		}

		for _, entry := range page.Data {
			assert.False(t, entry.CreatedAt.After(history(10)), "an entry is written at the time of the import")
		}
	})

	t.Run("a customer waits from their imported message until an imported reply", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.imported(t, nil)
		path := "/api/v1/conversations/" + id

		assert.Equal(t, history(0), timeAt(t, d.owner.GetJSON(path).AssertOK(), "data.waiting_since"))

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "On it", "created_at": stamp(3)}).AssertCreated()
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.waiting_since", nil).
			AssertJSONPath("data.status", "open")
	})

	t.Run("an imported conversation closes at the time given", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		res := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "body": "We reached out", "contact": testkit.JSON{"email": "c@example.com"},
			"created_at": stamp(0), "status": "closed", "closed_at": stamp(30), "author_id": d.ownerID(),
		}).AssertCreated()

		assert.Equal(t, history(30), timeAt(t, res, "data.closed_at"))
		assert.Equal(t, history(0), timeAt(t, res, "data.first_reply_at"))
		res.AssertJSONPath("data.status", "closed").AssertJSONPath("data.assignee", nil)
	})

	t.Run("sending the same external ids again finds the first copies", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.imported(t, testkit.JSON{"external_id": "zd-1"})

		again := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Where is my refund?",
			"contact": testkit.JSON{"email": "old@example.com"}, "created_at": stamp(0), "external_id": "zd-1",
		}).AssertOK().AssertHeader("Idempotent-Replayed", "true")
		assert.Equal(t, id, again.String("data.id"))

		path := "/api/v1/conversations/" + id + "/replies"
		reply := testkit.JSON{"body": "Done", "created_at": stamp(2), "external_id": "zd-1-r1"}
		first := d.owner.PostJSON(path, reply).AssertCreated().String("data.id")
		second := d.owner.PostJSON(path, reply).AssertOK().AssertHeader("Idempotent-Replayed", "true")
		assert.Equal(t, first, second.String("data.id"))

		d.owner.GetJSON("/api/v1/conversations/"+id).AssertOK().AssertJSONPath("data.message_count", 2)
	})

	t.Run("refuses times ahead, before 2000 and before the conversation's latest entry", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		create := func(at string) *testkit.Response {
			return d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
				"inbox_id": d.inboxID(), "author": "contact", "body": "Hi",
				"contact": testkit.JSON{"email": "x@example.com"}, "created_at": at,
			})
		}

		create(d.tc.Clock.Now().Add(time.Hour).UTC().Format(time.RFC3339)).AssertValidationError("created_at")
		create("1999-12-31T23:00:00Z").AssertValidationError("created_at")

		id := d.imported(t, nil)
		path := "/api/v1/conversations/" + id
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Late", "created_at": stamp(10)}).AssertCreated()
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Early", "created_at": stamp(5)}).
			AssertValidationError("created_at")

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "body": "Hi", "contact": testkit.JSON{"email": "y@example.com"},
			"created_at": stamp(10), "status": "closed", "closed_at": stamp(5),
		}).AssertValidationError("closed_at")
	})

	t.Run("only callers who may run imports bring history in", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, _ := d.join(t, enums.RoleMember)
		id := d.customer(t, "ann", "Live", "Hi")

		member.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Hi",
			"contact": testkit.JSON{"email": "z@example.com"}, "created_at": stamp(0),
		}).AssertForbidden()
		member.PostJSON("/api/v1/conversations/"+id+"/notes", testkit.JSON{
			"body": "Old note", "external_id": "n-1",
		}).AssertForbidden()
	})

	t.Run("reacts to nothing: no events, jobs, mail or notifications", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		fake := d.tc.Mail()
		created := record[appevents.ConversationCreated](d.tc)
		messages := record[appevents.MessageCreated](d.tc)
		closed := record[appevents.ConversationClosed](d.tc)
		assigned := record[appevents.ConversationAssigned](d.tc)
		d.tc.Drain()

		db := owned(t, d)
		count := func(table string) int {
			var n int
			require.NoError(t, db.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n))

			return n
		}
		notifications, runs, outbound := count("notifications"), count("workflow_runs"), count("outbound_emails")

		_, teammate := d.join(t, enums.RoleMember)
		jobs := count("river_job")

		id := d.imported(t, testkit.JSON{"assignee_id": memberID(teammate)})
		path := "/api/v1/conversations/" + id
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Still there?", "created_at": stamp(4)}).AssertCreated()
		d.owner.PostJSON(path+"/notes", testkit.JSON{
			"body": "Escalating", "created_at": stamp(5), "author_id": memberID(teammate),
		}).AssertCreated()
		d.owner.PostJSON(path+"/replies", testkit.JSON{
			"body": "Sorted", "created_at": stamp(6), "author_id": memberID(teammate), "then": "close",
		}).AssertCreated()

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.assignee.id", memberID(teammate)).
			AssertJSONPath("data.status", "closed")

		assert.Equal(t, jobs, count("river_job"), "no job is queued")
		d.tc.DrainLater()
		assert.Empty(t, fake.Sent())
		assert.Equal(t, notifications, count("notifications"))
		assert.Equal(t, runs, count("workflow_runs"))
		assert.Equal(t, outbound, count("outbound_emails"))
		assert.Empty(t, created.all())
		assert.Empty(t, messages.all())
		assert.Empty(t, closed.all())
		assert.Empty(t, assigned.all())
	})
}
