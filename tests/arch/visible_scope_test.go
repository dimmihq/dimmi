package arch_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
unscopedQueries are the queries that may list or find conversations
without the visibility function, each with the reason. Nothing else may:
a new list that forgets the function shows conversations a restricted
member must not see.
*/
func unscopedQueries() map[string]string {
	return map[string]string{
		"FindConversation":                  "one row by id: conversations.Find checks it with Scope.Contains",
		"FindConversationByNumber":          "one row by number: conversations.Find checks it with Scope.Contains",
		"ListConversations":                 "workspace-level readers only: no person to restrict",
		"ListTeamConversations":             "deleting a team moves all its conversations, whoever may see them",
		"CountTeamConversationsByStatus":    "warns what deleting a team moves: numbers only, names no conversation",
		"FindLatestThreadConversation":      "inbound routing: the mail decides the conversation, no member is asking",
		"FindRefConversations":              "inbound routing: the mail decides the conversation, no member is asking",
		"FindEmailThreadByToken":            "inbound routing: the mail decides the conversation, no member is asking",
		"FindSubjectThreadCandidates":       "inbound routing: the mail decides the conversation, no member is asking",
		"CountTaggedConversations":          "workspace-wide tag totals for managing tags: names no conversation",
		"ListPoolCandidates":                "counts each member's own load for assignment: names no conversation",
		"FindNextWaitingConversation":       "the assignment drain: the system picks, no member is asking",
		"ListWaitingInboxes":                "the assignment drain: names inboxes, not conversations",
		"ListWidgetConversations":           "a widget visitor's own conversations: the customer asks, not a member",
		"FindWidgetConversation":            "a widget visitor's own conversations: the customer asks, not a member",
		"CountWaitingConversations":         "inbox-wide queue totals for inbox settings: names no conversation",
		"CountInboxConversations":           "whether an inbox being deleted is empty: names no conversation",
		"CountContactConversations":         "whether a contact may be deleted without history: names no conversation",
		"ListStaleClosedConversations":      "the retention round: the system moves them, no member is asking",
		"CountStaleClosedConversations":     "a retention preview for settings: numbers only, names no conversation",
		"ListErasedMessageFiles":            "the erase job wipes a customer everywhere: no member is asking",
		"ListExportConversations":           "a privacy export covers all of a customer's data, for contacts:erase",
		"FindLatestContactConversationSpam": "inbound mail checks the sender's last verdict: no member is asking",
	}
}

var (
	queryName        = regexp.MustCompile(`(?m)^-- name: (\w+) :\w+\s*$`)
	readsTable       = regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+conversations\b`)
	lineComment      = regexp.MustCompile(`(?m)^\s*--.*$`)
	listingQueryName = regexp.MustCompile(`^(Filter|List|Count|Search|Find)`)
)

/*
sqlQueries splits a query file into the body of each named query.
*/
func sqlQueries(source string) map[string]string {
	out := map[string]string{}

	bounds := queryName.FindAllStringSubmatchIndex(source, -1)
	for i, b := range bounds {
		end := len(source)
		if i+1 < len(bounds) {
			end = bounds[i+1][0]
		}

		out[source[b[2]:b[3]]] = lineComment.ReplaceAllString(source[b[1]:end], "")
	}

	return out
}

/*
missingScope returns the listing queries of the source that read
conversations without the visibility function and are not on the
allowlist, sorted.
*/
func missingScope(source string, allowed map[string]string) []string {
	var out []string

	for name, body := range sqlQueries(source) {
		if !listingQueryName.MatchString(name) || !readsTable.MatchString(body) {
			continue
		}

		if strings.Contains(body, "dimmi_conversation_visible") {
			continue
		}

		if _, ok := allowed[name]; !ok {
			out = append(out, name)
		}
	}

	sort.Strings(out)

	return out
}

/*
queryFiles reads every query file of the application.
*/
func queryFiles(t *testing.T) map[string]string {
	t.Helper()

	dir := os.DirFS(filepath.Join("..", "..", "database", "queries"))

	paths, err := fs.Glob(dir, "*.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	out := map[string]string{}

	for _, path := range paths {
		source, err := fs.ReadFile(dir, path)
		require.NoError(t, err)

		out[path] = string(source)
	}

	return out
}

func TestVisibleScope(t *testing.T) {
	t.Parallel()

	t.Run("every query listing conversations calls the visibility function or says why not", func(t *testing.T) {
		t.Parallel()

		for name, source := range queryFiles(t) {
			assert.Empty(t, missingScope(source, unscopedQueries()), name)
		}
	})

	t.Run("a planted query without the function fails", func(t *testing.T) {
		t.Parallel()

		planted := `
-- name: ListStaleConversations :many
-- ListStaleConversations mentions dimmi_conversation_visible only in this comment.
SELECT * FROM conversations WHERE workspace_id = @workspace_id;

-- name: CountWaitingByInbox :many
SELECT c.inbox_id, count(*) FROM inboxes i JOIN conversations c ON c.inbox_id = i.id GROUP BY 1;

-- name: ListScopedConversations :many
SELECT * FROM conversations c WHERE dimmi_conversation_visible(c.assignee_member_id, c.assignee_team_id,
    @access_mode::text, sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean);

-- name: ListTeams :many
SELECT * FROM teams;

-- name: LockConversation :one
SELECT * FROM conversations FOR UPDATE;
`

		assert.Equal(t, []string{"CountWaitingByInbox", "ListStaleConversations"},
			missingScope(planted, unscopedQueries()))
	})

	t.Run("every allowlisted query still exists and still needs its place", func(t *testing.T) {
		t.Parallel()

		known := map[string]string{}
		for _, source := range queryFiles(t) {
			maps.Copy(known, sqlQueries(source))
		}

		for name := range unscopedQueries() {
			body, ok := known[name]
			require.True(t, ok, name)
			assert.Regexp(t, readsTable, body, name)
			assert.NotContains(t, body, "dimmi_conversation_visible", name)
		}
	})
}
