package inbox_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

const savedReplies = "/api/v1/saved-replies"

func TestSavedReplyAPI(t *testing.T) {
	t.Parallel()

	t.Run("adds, searches, changes and removes a personal reply", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		id := d.Owner.PostJSON(savedReplies, testkit.JSON{
			"title": " Refund policy ", "body": "Hi {{ contact.first_name | default: \"there\" }}",
			"shortcut": "#refund",
		}).AssertCreated().
			AssertJSONPath("data.object", "saved_reply").
			AssertJSONPath("data.title", "Refund policy").
			AssertJSONPath("data.shortcut", "refund").
			AssertJSONPath("data.shared", false).
			AssertJSONPath("data.actions.tag_ids.#", 0).String("data.id")
		d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "Welcome", "body": "Hello", "shared": true}).
			AssertCreated().AssertJSONPath("data.shared", true).AssertJSONPath("data.owner_id", nil)

		d.Owner.GetJSON(savedReplies).AssertOK().AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.title", "Refund policy")
		d.Owner.GetJSON(savedReplies+"?q=REF").AssertOK().AssertJSONPath("data.#", 1)

		d.Owner.PatchJSON(savedReplies+"/"+id, testkit.JSON{"title": "Refunds", "shortcut": nil}).AssertOK().
			AssertJSONPath("data.title", "Refunds").AssertJSONPath("data.shortcut", nil)
		d.Owner.GetJSON(savedReplies+"/"+id).AssertOK().AssertJSONPath("data.title", "Refunds")

		d.Owner.Delete(savedReplies + "/" + id).AssertNoContent()
		d.Owner.Delete(savedReplies + "/" + id).AssertNotFound()
		d.Owner.GetJSON(savedReplies + "/rpl_bogus").AssertNotFound()
	})

	t.Run("refuses an empty reply and a shortcut already used", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "  ", "body": "x"}).
			AssertStatus(422).AssertJSONPath("code", "saved_reply_title_required")
		d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "A", "body": "   "}).
			AssertStatus(422).AssertJSONPath("code", "saved_reply_body_required")
		d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "A", "body": "a", "shortcut": "hi"}).AssertCreated()
		d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "B", "body": "b", "shortcut": "HI"}).
			AssertStatus(409).AssertJSONPath("code", "saved_reply_shortcut_taken")
	})

	t.Run("keeps a macro's actions and refuses unknown ones", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, member := d.Join(t, enums.RoleMember)
		assignee := models.PublicID(models.PrefixMember, member.ID)
		tag := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Refund"}).AssertCreated().String("data.id")

		id := d.Owner.PostJSON(savedReplies, testkit.JSON{
			"title": "Close as refunded", "body": "Done!",
			"actions": testkit.JSON{
				"status": "snoozed", "snooze_preset": "tomorrow", "assignee_id": assignee, "tag_ids": []string{tag},
				"priority": "high",
			},
		}).AssertCreated().
			AssertJSONPath("data.actions.status", "snoozed").
			AssertJSONPath("data.actions.snooze_preset", "tomorrow").
			AssertJSONPath("data.actions.assignee_id", assignee).
			AssertJSONPath("data.actions.tag_ids.0", tag).
			AssertJSONPath("data.actions.priority", "high").String("data.id")

		d.Owner.PatchJSON(savedReplies+"/"+id, testkit.JSON{"actions": testkit.JSON{"status": "closed"}}).AssertOK().
			AssertJSONPath("data.actions.status", "closed").AssertJSONPath("data.actions.tag_ids.#", 0)

		d.Owner.PostJSON(savedReplies, testkit.JSON{
			"title": "x", "body": "x", "actions": testkit.JSON{"status": "snoozed"},
		}).AssertStatus(422).AssertJSONPath("code", "invalid_macro")
		d.Owner.PostJSON(savedReplies, testkit.JSON{
			"title": "x", "body": "x", "actions": testkit.JSON{"tag_ids": []string{"tag_bogus"}},
		}).AssertStatus(422).AssertJSONPath("code", "invalid_macro")
		d.Owner.PostJSON(savedReplies, testkit.JSON{
			"title": "x", "body": "x", "actions": testkit.JSON{"team_id": "team_01jc0000000000000000000000"},
		}).AssertStatus(422).AssertJSONPath("code", "invalid_macro")
	})

	t.Run("hides personal replies from teammates and shares only with permission", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)

		mine := d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "Mine", "body": "x"}).AssertCreated().
			String("data.id")
		shared := d.Owner.PostJSON(savedReplies, testkit.JSON{"title": "Ours", "body": "x", "shared": true}).
			AssertCreated().String("data.id")

		member.GetJSON(savedReplies).AssertOK().AssertJSONPath("data.#", 1).AssertJSONPath("data.0.id", shared)
		member.GetJSON(savedReplies + "/" + mine).AssertNotFound()
		member.PatchJSON(savedReplies+"/"+mine, testkit.JSON{"title": "x"}).AssertNotFound()
		member.PatchJSON(savedReplies+"/"+shared, testkit.JSON{"title": "x"}).AssertForbidden()
		member.Delete(savedReplies + "/" + shared).AssertForbidden()
		member.PostJSON(savedReplies, testkit.JSON{"title": "T", "body": "x", "shared": true}).AssertForbidden()

		own := member.PostJSON(savedReplies, testkit.JSON{"title": "Theirs", "body": "x"}).AssertCreated().
			String("data.id")
		member.PatchJSON(savedReplies+"/"+own, testkit.JSON{"shared": true}).AssertForbidden()
		d.Owner.PatchJSON(savedReplies+"/"+mine, testkit.JSON{"shared": true}).AssertOK().
			AssertJSONPath("data.shared", true)
		member.GetJSON(savedReplies).AssertOK().AssertJSONPath("data.#", 3)
	})

	t.Run("imports rows all or none", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		imports := "/api/v1/saved-reply-imports"

		d.Owner.PostJSON(imports, testkit.JSON{"replies": []testkit.JSON{
			{"title": "One", "body": "1"}, {"title": "Two", "body": "2", "shared": true},
		}}).AssertCreated().AssertJSONPath("data.#", 2).AssertJSONPath("data.1.shared", true)

		d.Owner.PostJSON(imports, testkit.JSON{"replies": []testkit.JSON{
			{"title": "Three", "body": "3", "shortcut": "s"}, {"title": "Four", "body": "4", "shortcut": "s"},
		}}).AssertStatus(409)
		d.Owner.GetJSON(savedReplies).AssertOK().AssertJSONPath("data.#", 2)
		d.Owner.PostJSON(imports, testkit.JSON{"replies": []testkit.JSON{}}).AssertValidationError("replies")
	})
}
