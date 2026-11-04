package conversations_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
	"github.com/dimmihq/dimmi/database/migrations"
)

/*
owned connects to the test's database as its owner, which row security
does not narrow, for checks and seeding that no request makes.
*/
func owned(t *testing.T, d desk) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL: d.tc.Database.URL, MaxConns: 2, PoolMode: database.PoolModeSession, TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
conversationID reads a public conversation id.
*/
func conversationID(t *testing.T, d desk, id string) ids.UUID {
	t.Helper()

	return decode(t, d, models.PrefixConversation, id)
}

func TestConversationInboxColumns(t *testing.T) {
	t.Parallel()

	t.Run("search folds accents and stems alike in documents and queries", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		db := owned(t, d)

		var matches bool
		require.NoError(t, db.QueryRow(t.Context(),
			`SELECT to_tsvector('dimmi_search', 'Rückerstattungen') @@ to_tsquery('dimmi_search', 'ruckerstattungen')`).
			Scan(&matches))
		assert.True(t, matches)

		require.NoError(t, db.QueryRow(t.Context(),
			`SELECT dimmi_unaccent('Müller') = 'Muller'`).Scan(&matches))
		assert.True(t, matches)

		id := d.customer(t, "ann", "Rückerstattung", "Ich möchte eine Überweisung")
		search := "/api/v1/conversations?filter[view]=all&q="
		assert.Equal(t, []string{id}, listed(d.owner.GetJSON(search+"ruckerstattung")))
		assert.Equal(t, []string{id}, listed(d.owner.GetJSON(search+"uberweisung")))
	})

	t.Run("a customer waits from their first unanswered message until someone answers", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Refund", "Please")
		path := "/api/v1/conversations/" + id

		since := d.owner.GetJSON(path).AssertOK().String("data.waiting_since")
		assert.NotEmpty(t, since)

		d.tc.Clock.Travel(time.Minute)
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Anyone?"}).AssertCreated()
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.waiting_since", since)

		d.tc.Clock.Travel(time.Minute)
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Looking"}).AssertCreated()
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.waiting_since", since)

		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "tomorrow"}).AssertOK()
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.waiting_since", since)
		d.owner.PatchJSON(path, testkit.JSON{"status": "open"}).AssertOK()

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Done"}).AssertCreated()
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.waiting_since", nil)
	})

	t.Run("closing stops the wait", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please")

		d.owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()
		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.waiting_since", nil)
	})

	t.Run("a note moves the activity time and leaves the message time", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please")
		before := d.owner.GetJSON(path).AssertOK()
		message := before.String("data.last_message_at")

		d.tc.Clock.Travel(time.Hour)
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Looking"}).AssertCreated()

		after := d.owner.GetJSON(path).AssertOK()
		assert.Equal(t, message, after.String("data.last_message_at"))
		assert.NotEqual(t, before.String("data.last_activity_at"), after.String("data.last_activity_at"))
	})

	t.Run("a message remembers its connection and the conversation its latest message", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Refund", "Please")
		path := "/api/v1/conversations/" + id
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Looking"}).AssertCreated()

		db := owned(t, d)

		var pointed, connected, notes int
		require.NoError(t, db.QueryRow(t.Context(), `
			SELECT
			    count(*) FILTER (WHERE c.last_message_id = m.id AND m.type = 'message'),
			    count(*) FILTER (WHERE m.type = 'message' AND m.channel_connection_id = c.channel_connection_id),
			    count(*) FILTER (WHERE m.type = 'note' AND m.channel_connection_id IS NULL)
			FROM messages m JOIN conversations c ON c.id = m.conversation_id WHERE c.id = $1`,
			conversationID(t, d, id)).Scan(&pointed, &connected, &notes))
		assert.Equal(t, 1, pointed)
		assert.Equal(t, 1, connected)
		assert.Equal(t, 1, notes)
	})

	t.Run("the migration's backfill finds who waits and what happened last", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		waiting := conversationID(t, d, d.customer(t, "ann", "A", "a"))
		answered := conversationID(t, d, d.customer(t, "bea", "B", "b"))
		closed := conversationID(t, d, d.customer(t, "cid", "C", "c"))
		d.owner.PostJSON(
			"/api/v1/conversations/"+models.PublicID(models.PrefixConversation, answered)+"/replies",
			testkit.JSON{"body": "Done"},
		).AssertCreated()
		d.owner.PatchJSON(
			"/api/v1/conversations/"+models.PublicID(models.PrefixConversation, closed),
			testkit.JSON{"status": "closed"},
		).AssertOK()

		db := owned(t, d)
		_, err := db.Exec(t.Context(),
			`UPDATE conversations SET waiting_since = NULL, last_activity_at = NULL, last_message_id = NULL`)
		require.NoError(t, err)

		for _, statement := range backfill(t) {
			_, err := db.Exec(t.Context(), statement)
			require.NoError(t, err)
		}

		rows := map[ids.UUID]struct {
			waiting, activity, last bool
		}{}

		found, err := db.Query(t.Context(), `SELECT id, waiting_since IS NOT NULL, last_activity_at IS NOT NULL,
			last_message_id IS NOT NULL FROM conversations`)
		require.NoError(t, err)
		defer found.Close()

		for found.Next() {
			var (
				id ids.UUID
				r  struct{ waiting, activity, last bool }
			)
			require.NoError(t, found.Scan(&id, &r.waiting, &r.activity, &r.last))
			rows[id] = r
		}

		assert.True(t, rows[waiting].waiting)
		assert.False(t, rows[answered].waiting)
		assert.False(t, rows[closed].waiting)
		assert.True(t, rows[waiting].activity && rows[waiting].last)
	})
}

/*
backfill returns the statements of the migration that fill the inbox
columns.
*/
func backfill(t *testing.T) []string {
	t.Helper()

	text, err := fs.ReadFile(migrations.Files(), "20261201030003_add_inbox_columns_to_conversations.sql")
	require.NoError(t, err)

	up, _, ok := strings.Cut(string(text), "-- +goose Down")
	require.True(t, ok)

	_, tail, ok := strings.Cut(up, "UPDATE conversations\n")
	require.True(t, ok)

	var statements []string

	for statement := range strings.SplitSeq("UPDATE conversations\n"+tail, ";") {
		if strings.TrimSpace(statement) != "" {
			statements = append(statements, statement)
		}
	}

	return statements
}

func TestConversationListPlans(t *testing.T) {
	t.Parallel()

	d := open(t)
	_, member := d.join(t, enums.RoleMember)
	first := d.customer(t, "ann", "Seed", "seed")
	db := owned(t, d)
	workspace := d.tc.Workspace.Workspace.ID
	me := member.ID

	_, err := db.Exec(t.Context(), `
		INSERT INTO conversations (workspace_id, number, inbox_id, channel_connection_id, contact_id, status, spam,
			priority, assignee_member_id, assignee_team_id, snoozed_until, waiting_since, last_message_at, created_at)
		SELECT c.workspace_id, 100 + g, c.inbox_id, c.channel_connection_id, c.contact_id,
			CASE WHEN g % 5 = 0 THEN 'closed' WHEN g % 5 = 1 THEN 'snoozed' ELSE 'open' END,
			g % 50 = 0, (g % 5)::smallint, CASE WHEN g % 3 = 0 THEN $2::uuid END, NULL,
			CASE WHEN g % 5 = 1 THEN now() + g * interval '1 minute' END,
			CASE WHEN g % 2 = 0 THEN now() - g * interval '1 minute' END,
			now() - g * interval '1 minute', now() - g * interval '1 minute'
		FROM conversations c, generate_series(1, 10000) g WHERE c.id = $1`, conversationID(t, d, first), me)
	require.NoError(t, err)

	_, err = db.Exec(t.Context(), `ANALYZE conversations`)
	require.NoError(t, err)

	base := querybuilder.Conversations{
		WorkspaceID: workspace, MemberID: me, Statuses: []string{"open"}, Limit: 25,
		Access: querybuilder.Access{Mode: "all"},
	}
	assigned := base
	assigned.AssigneeID = &me
	waiting := base
	waiting.Sort, waiting.Waiting = querybuilder.SortWaiting, true
	created := base
	created.Statuses, created.Sort = nil, querybuilder.SortNewestStarted
	snoozed := base
	snoozed.Statuses, snoozed.Sort = []string{"snoozed"}, querybuilder.SortSnoozeEnds
	spam := base
	spam.Statuses, spam.Spam = nil, true
	urgent := base
	urgent.Sort, urgent.PriorityMin = querybuilder.SortPriority, new(int16(1))
	deleted := querybuilder.Conversations{WorkspaceID: workspace, MemberID: me, Deleted: true, Limit: 25}

	for name, tc := range map[string]struct {
		list  querybuilder.Conversations
		index string
	}{
		"all by latest activity": {base, "conversations_all_activity_idx"},
		"a member's":             {assigned, "conversations_member_activity_idx"},
		"waiting longest":        {waiting, "conversations_waiting_idx"},
		"newest started":         {created, "conversations_created_idx"},
		"snooze ends":            {snoozed, "conversations_snooze_idx"},
		"spam":                   {spam, "conversations_spam_idx"},
		"priority":               {urgent, "conversations_priority_idx"},
		"deleted":                {deleted, "conversations_deleted_idx"},
	} {
		t.Run(fmt.Sprintf("%s walks its index", name), func(t *testing.T) {
			t.Parallel()

			sql, args := tc.list.IDs()

			rows, err := db.Query(t.Context(), "EXPLAIN "+sql, args...)
			require.NoError(t, err)
			defer rows.Close()

			var plan strings.Builder

			for rows.Next() {
				var line string
				require.NoError(t, rows.Scan(&line))
				plan.WriteString(line + "\n")
			}

			assert.Contains(t, plan.String(), tc.index)
		})
	}
}
