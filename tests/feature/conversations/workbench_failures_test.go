package conversations_test

import (
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
openView saves a personal view on any open conversation and returns its
path.
*/
func openView(d desk) string {
	return "/api/v1/conversation-views/" + d.owner.PostJSON("/api/v1/conversation-views", testkit.JSON{
		"name": "Open", "scope": "personal", "definition": oneCondition("status", "in", []string{"open"}),
	}).AssertCreated().String("data.id")
}

/*
redraft is the body of a later save of a draft.
*/
func redraft(d desk) testkit.JSON {
	return testkit.JSON{"body": "y", "client_updated_at": d.tc.Clock.Now()}
}

/*
replyDraft saves a reply draft on the conversation and returns its path.
*/
func replyDraft(d desk, conv string) string {
	return draftsPath + "/" + d.owner.PostJSON(draftsPath, draft(conv, "reply", "x", d.tc.Clock.Now())).
		AssertCreated().String("data.id")
}

func TestWorkbenchStorageFailures(t *testing.T) {
	t.Parallel()

	options := testkit.JSON{"options": []testkit.JSON{{"key": "soon", "label": "Soon", "kind": "in", "minutes": 30}}}

	for _, tc := range []struct {
		name   string
		breaks func(d desk)
		step   func(d desk, conv string) request
	}{
		{"lists drafts when they are gone", breakTable("conversation_drafts"), fixed("GET", draftsPath, nil)},
		{
			"lists a conversation's drafts when conversations are gone", breakTable("conversations"),
			func(_ desk, conv string) request {
				return request{"GET", draftsPath + "?filter[conversation]=" + conv, nil}
			},
		},
		{
			"saves drafts when they cannot be written", failWrites("conversation_drafts", "INSERT"),
			func(d desk, conv string) request {
				return request{"POST", draftsPath, draft(conv, "reply", "x", d.tc.Clock.Now())}
			},
		},
		{
			"saves drafts when they are gone", breakTable("conversation_drafts"),
			func(d desk, conv string) request {
				return request{"POST", draftsPath, draft(conv, "reply", "x", d.tc.Clock.Now())}
			},
		},
		{
			"saves drafts with files when files are gone", breakTable("attachments"),
			func(d desk, conv string) request {
				body := draft(conv, "reply", "x", d.tc.Clock.Now())
				body["attachment_ids"] = []string{models.PublicID(models.PrefixAttachment, ids.New())}

				return request{"POST", draftsPath, body}
			},
		},
		{
			"changes drafts when they cannot be written", failWrites("conversation_drafts", "UPDATE"),
			func(d desk, conv string) request {
				return request{"PATCH", replyDraft(d, conv), redraft(d)}
			},
		},
		{
			"changes drafts when they are gone", breakTable("conversation_drafts"),
			func(d desk, conv string) request {
				return request{"PATCH", replyDraft(d, conv), redraft(d)}
			},
		},
		{
			"discards drafts when they cannot be deleted", failWrites("conversation_drafts", "DELETE"),
			func(d desk, conv string) request { return request{"DELETE", replyDraft(d, conv), nil} },
		},
		{
			"replies when the draft cannot be ended", failWrites("conversation_drafts", "DELETE"),
			on("POST", "/replies", testkit.JSON{"body": "x"}),
		},
		{
			"lists views when they are gone", breakTable("conversation_views"),
			fixed("GET", "/api/v1/conversation-views", nil),
		},
		{
			"shows a view when views are gone", breakTable("conversation_views"),
			func(d desk, _ string) request { return request{"GET", openView(d), nil} },
		},
		{
			"saves views when they cannot be written", failWrites("conversation_views", "INSERT"),
			fixed("POST", "/api/v1/conversation-views", testkit.JSON{
				"name": "Open", "scope": "personal", "definition": oneCondition("status", "in", []string{"open"}),
			}),
		},
		{
			"renames views when they cannot be written", failWrites("conversation_views", "UPDATE"),
			func(d desk, _ string) request { return request{"PATCH", openView(d), testkit.JSON{"name": "Other"}} },
		},
		{
			"deletes views when they cannot be written", failWrites("conversation_views", "UPDATE"),
			func(d desk, _ string) request { return request{"DELETE", openView(d), nil} },
		},
		{
			"orders views when they are gone", breakTable("conversation_views"),
			fixed("PATCH", "/api/v1/conversation-views", testkit.JSON{
				"order": []string{"cvw_01jbd0k3a8x9t2m4q5r6s7v8w9"},
			}),
		},
		{
			"lists a view's conversations when views are gone", breakTable("conversation_views"),
			func(d desk, _ string) request {
				view := strings.TrimPrefix(openView(d), "/api/v1/conversation-views/")

				return request{"GET", "/api/v1/conversations?filter[view]=" + view, nil}
			},
		},
		{
			"shows preferences when they are gone", breakTable("member_inbox_preferences"),
			fixed("GET", "/api/v1/inbox-preferences", nil),
		},
		{
			"saves preferences when they cannot be written", failWrites("member_inbox_preferences", "INSERT"),
			fixed("PATCH", "/api/v1/inbox-preferences", testkit.JSON{"pinned_attributes": []string{"plan"}}),
		},
		{
			"lists snooze lists when they are gone", breakTable("snooze_lists"),
			fixed("GET", "/api/v1/snooze-lists", nil),
		},
		{
			"sets snooze lists when they cannot be written", failWrites("snooze_lists", "INSERT"),
			fixed("PATCH", "/api/v1/snooze-lists/workspace", options),
		},
		{
			"removes snooze lists when they cannot be deleted", failWrites("snooze_lists", "DELETE"),
			fixed("PATCH", "/api/v1/snooze-lists/workspace", testkit.JSON{"options": nil}),
		},
		{
			"sets a team's snooze list when teams are gone", breakTable("teams"),
			fixed("PATCH", "/api/v1/snooze-lists/team_01jbd0k3a8x9t2m4q5r6s7v8w9", options),
		},
		{
			"offers snooze choices when lists are gone", breakTable("snooze_lists"),
			func(_ desk, conv string) request {
				return request{"GET", "/api/v1/snooze-options?conversation=" + conv, nil}
			},
		},
		{
			"counts pinned views when views are gone", breakTable("conversation_views"),
			func(d desk, _ string) request {
				view := strings.TrimPrefix(openView(d), "/api/v1/conversation-views/")
				d.owner.PatchJSON("/api/v1/inbox-preferences", testkit.JSON{
					"sidebar": testkit.JSON{"pinned_views": []string{view}},
				}).AssertOK()

				return request{"GET", "/api/v1/conversation-counts", nil}
			},
		},
		{
			"shows preferences naming a team when teams are gone", breakTable("teams"),
			func(d desk, _ string) request {
				team := d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().
					String("data.id")
				d.owner.PatchJSON("/api/v1/inbox-preferences", testkit.JSON{
					"views": testkit.JSON{team: testkit.JSON{"sort": "created_at"}},
				}).AssertOK()

				return request{"GET", "/api/v1/inbox-preferences", nil}
			},
		},
		{
			"offers snooze choices when members are gone", breakTable("workspace_members"),
			func(_ desk, conv string) request {
				return request{"GET", "/api/v1/snooze-options?conversation=" + conv, nil}
			},
		},
		{
			"orders shared views when they cannot be written", failWrites("conversation_views", "UPDATE"),
			func(d desk, _ string) request {
				id := d.owner.PostJSON("/api/v1/conversation-views", testkit.JSON{
					"name": "Shared", "scope": "shared", "definition": oneCondition("status", "in", []string{"open"}),
				}).AssertCreated().String("data.id")

				return request{"PATCH", "/api/v1/conversation-views", testkit.JSON{"order": []string{id}}}
			},
		},
		{
			"snoozes by option when lists are gone", breakTable("snooze_lists"),
			on("PATCH", "", testkit.JSON{"status": "snoozed", "snooze_option": "tomorrow"}),
		},
	} {
		t.Run("fails cleanly: "+tc.name, func(t *testing.T) {
			t.Parallel()
			d := open(t)
			conv := d.customer(t, "ann", "Billing", "hello there")
			s := tc.step(d, conv)
			tc.breaks(d)

			d.owner.Send(s.method, s.path, s.body).AssertStatus(500)
		})
	}
}
