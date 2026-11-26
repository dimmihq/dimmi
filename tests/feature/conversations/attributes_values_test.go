package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
attribute makes a conversation attribute and returns its id.
*/
func (d desk) attribute(t *testing.T, body testkit.JSON) string {
	t.Helper()

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	d.owner.PostJSON("/api/v1/conversation-attributes", body).AssertCreated().Decode(&out)

	return out.Data.ID
}

/*
valueOf reads one attribute's value and who set it off a conversation.
*/
func valueOf(t *testing.T, res *testkit.Response, key string) (any, map[string]any) {
	t.Helper()

	var body struct {
		Data struct {
			Values []struct {
				Attribute struct {
					Key string `json:"key"`
				} `json:"attribute"`
				Value any            `json:"value"`
				SetBy map[string]any `json:"set_by"`
			} `json:"values"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, v := range body.Data.Values {
		if v.Attribute.Key == key {
			return v.Value, v.SetBy
		}
	}

	t.Fatalf("no attribute %s", key)

	return nil, nil
}

func TestConversationAttributeValues(t *testing.T) {
	t.Parallel()

	t.Run("a teammate sets values of every type, and a null clears one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		seen := record[appevents.ConversationAttributeChanged](d.tc)

		d.attribute(t, testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing", "Bug"},
		})
		d.attribute(t, testkit.JSON{
			"key": "areas", "label": "Areas", "data_type": "multiple_choice", "options": []string{"API", "Inbox"},
		})
		d.attribute(t, testkit.JSON{"key": "seats", "label": "Seats", "data_type": "number"})
		d.attribute(t, testkit.JSON{"key": "refund", "label": "Refund", "data_type": "decimal"})
		d.attribute(t, testkit.JSON{"key": "follow_up", "label": "Follow up", "data_type": "datetime"})
		d.attribute(t, testkit.JSON{"key": "notes", "label": "Notes", "data_type": "long_text"})

		conv := d.customer(t, "ann", "Refund", "Please")
		path := "/api/v1/conversations/" + conv + "/attributes"

		res := d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{
			"issue": "Billing", "areas": []string{"API", "API", "Inbox"}, "seats": 12, "refund": 19.5,
			"follow_up": "2026-12-01T10:30:00+02:00", "notes": "Line one\nLine two",
		}}).AssertOK()

		v, by := valueOf(t, res, "issue")
		assert.Equal(t, "Billing", v)
		assert.Equal(t, "member", by["by"])
		assert.Equal(t, d.ownerID(), by["id"])
		v, _ = valueOf(t, res, "areas")
		assert.Equal(t, []any{"API", "Inbox"}, v, "each choice once")
		v, _ = valueOf(t, res, "follow_up")
		assert.Equal(t, "2026-12-01T08:30:00Z", v, "kept in UTC")
		assert.Len(t, seen.all(), 6)

		res = d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{"issue": nil}}).AssertOK()
		v, _ = valueOf(t, res, "issue")
		assert.Nil(t, v)

		events := seen.all()
		require.Len(t, events, 7)
		assert.Equal(t, "issue", events[6].Key)
		assert.Equal(t, "Billing", events[6].From)
		assert.Nil(t, events[6].To)
		assert.Equal(t, "member", events[6].By)

		var timeline struct {
			Data []struct {
				Type  string         `json:"type"`
				Event map[string]any `json:"event"`
			} `json:"data"`
		}
		d.owner.GetJSON("/api/v1/conversations/" + conv + "/messages").AssertOK().Decode(&timeline)

		changes := 0
		for _, m := range timeline.Data {
			if m.Type == "event" && m.Event["name"] == "conversation.attribute_changed" {
				changes++
			}
		}

		assert.Equal(t, 7, changes, "every change joins the timeline")
	})

	t.Run("refuses values that do not fit, setting none of them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.attribute(t, testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing"},
		})
		d.attribute(t, testkit.JSON{"key": "seats", "label": "Seats", "data_type": "number"})
		d.attribute(t, testkit.JSON{"key": "owner", "label": "Account", "data_type": "company"})
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please") + "/attributes"

		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{
			"issue": "Billing", "seats": 1.5, "owner": "ctc_01jz00000000000000000000", "nope": 1,
		}}).AssertStatus(422).
			AssertJSONPath("code", "attribute_values_invalid").
			AssertJSONPath("problems.seats", "not_whole").
			AssertJSONPath("problems.owner", "type_mismatch").
			AssertJSONPath("problems.nope", "unknown_attribute")

		v, _ := valueOf(t, d.owner.GetJSON(path).AssertOK(), "issue")
		assert.Nil(t, v, "nothing was set")
	})

	t.Run("a teammate cannot set what only the AI fills, and an archived one stops taking values", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.attribute(t, testkit.JSON{
			"key": "mood", "label": "Mood", "data_type": "choice", "options": []string{"calm", "upset"},
			"filled_by": "ai",
		})
		id := d.attribute(t, testkit.JSON{"key": "plan", "label": "Plan", "data_type": "text"})
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please") + "/attributes"

		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{"mood": "calm"}}).AssertStatus(422).
			AssertJSONPath("code", "attribute_filled_by_ai")

		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{"plan": "Pro"}}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversation-attributes/"+id, testkit.JSON{"status": "archived"}).AssertOK()
		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{"plan": "Team"}}).AssertStatus(422).
			AssertJSONPath("problems.plan", "archived_attribute")

		v, _ := valueOf(t, d.owner.GetJSON(path).AssertOK(), "plan")
		assert.Equal(t, "Pro", v, "an archived attribute keeps its last value")
	})

	t.Run("refuses the AI filling types it cannot read", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "proof", "label": "Proof", "data_type": "file", "filled_by": "ai",
		}).AssertStatus(422).AssertJSONPath("code", "filled_by_invalid")
		d.owner.PostJSON("/api/v1/conversation-attributes", testkit.JSON{
			"key": "areas", "label": "Areas", "data_type": "multiple_choice",
		}).AssertStatus(422).AssertJSONPath("code", "options_invalid")
	})

	t.Run("a conversation starts and imports with values", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.attribute(t, testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing", "Bug"},
			"filled_by": "ai",
		})

		var started struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "contact": testkit.JSON{"email": "imported@example.com"}, "body": "Hello",
			"author":      "contact",
			"external_id": "ic_1", "created_at": "2026-01-02T10:00:00Z", "status": "closed",
			"attributes": testkit.JSON{"issue": "Bug"},
		}).AssertCreated().Decode(&started)

		v, by := valueOf(t, d.owner.GetJSON("/api/v1/conversations/"+started.Data.ID+"/attributes").AssertOK(),
			"issue")
		assert.Equal(t, "Bug", v, "an import sets even what only the AI fills")
		assert.Equal(t, "import", by["by"])

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "contact": testkit.JSON{"email": "live@example.com"}, "body": "Hello",
			"author":     "contact",
			"attributes": testkit.JSON{"issue": "Nope"},
		}).AssertStatus(422).AssertJSONPath("code", "attribute_filled_by_ai")
	})
}

func TestConversationAttributeFiles(t *testing.T) {
	t.Parallel()

	t.Run("a file attribute holds uploads, shows them, and refuses one the workspace lacks", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.attribute(t, testkit.JSON{"key": "proof", "label": "Proof", "data_type": "file"})
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Please") + "/attributes"

		file := d.owner.WithHeader("Content-Type", "text/plain").
			Send(fhttp.MethodPost, "/api/v1/attachments?filename=receipt.txt", []byte("paid")).AssertCreated().
			String("data.id")

		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{"proof": []string{file}}}).AssertOK().
			AssertJSONPath("data.values.0.value.0", file).
			AssertJSONPath("data.values.0.files.0.id", file).
			AssertJSONPath("data.values.0.files.0.filename", "receipt.txt")

		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{
			"proof": []string{ids.Encode("att", ids.New())},
		}}).AssertStatus(422).AssertJSONPath("problems.proof", "unknown_file")
	})
}

func TestConversationAttributesRequiredToClose(t *testing.T) {
	t.Parallel()

	t.Run("a teammate cannot close, alone or in bulk, while a required attribute is empty", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.attribute(t, testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing", "Bug"},
			"required_to_close": true,
		})
		d.attribute(t, testkit.JSON{"key": "notes", "label": "Notes", "data_type": "text"})

		one, two := d.customer(t, "ann", "Refund", "Please"), d.customer(t, "bo", "Bug", "Broken")

		d.owner.PatchJSON("/api/v1/conversations/"+one, testkit.JSON{"status": "closed"}).AssertStatus(422).
			AssertJSONPath("code", "conversation_attributes_required").
			AssertJSONPath("missing.0.key", "issue").
			AssertJSONPath("missing.0.label", "Issue type")

		d.owner.PatchJSON("/api/v1/conversations/"+one+"/attributes", testkit.JSON{
			"values": testkit.JSON{"issue": "Billing"},
		}).AssertOK()

		d.owner.PostJSON("/api/v1/conversation-bulk-changes", testkit.JSON{
			"conversation_ids": []string{one, two}, "changes": testkit.JSON{"status": "closed"},
		}).AssertOK().
			AssertJSONPath("data.0.ok", true).
			AssertJSONPath("data.1.error", "conversation_attributes_required")

		d.owner.GetJSON("/api/v1/conversations/"+one).AssertOK().AssertJSONPath("data.status", "closed")
		d.owner.GetJSON("/api/v1/conversations/"+two).AssertOK().AssertJSONPath("data.status", "open")
	})
}

func TestConversationAttributeTeams(t *testing.T) {
	t.Parallel()

	t.Run("an attribute shown only to other teams is hidden, refused and never required", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		var team struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().Decode(&team)

		d.attribute(t, testkit.JSON{
			"key": "invoice", "label": "Invoice", "data_type": "text", "required_to_close": true,
			"team_ids": []string{team.Data.ID},
		})
		d.attribute(t, testkit.JSON{"key": "notes", "label": "Notes", "data_type": "text"})

		conv := d.customer(t, "ann", "Refund", "Please")
		path := "/api/v1/conversations/" + conv + "/attributes"

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.values.#", 1).
			AssertJSONPath("data.values.0.attribute.key", "notes")
		d.owner.PatchJSON(path, testkit.JSON{"values": testkit.JSON{"invoice": "INV-1"}}).AssertStatus(422).
			AssertJSONPath("problems.invoice", "unknown_attribute")
		d.owner.PatchJSON("/api/v1/conversations/"+conv, testkit.JSON{"status": "closed"}).AssertOK()
	})
}

func TestConversationAttributeSettings(t *testing.T) {
	t.Parallel()

	t.Run("keeps who fills it, required to close and its teams; old routes still answer", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		var team struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().Decode(&team)

		id := d.attribute(t, testkit.JSON{
			"key": "issue", "label": "Issue type", "data_type": "choice", "options": []string{"Billing"},
			"filled_by": "teammates", "required_to_close": true, "team_ids": []string{team.Data.ID},
		})

		d.owner.GetJSON("/api/v1/conversation-attributes").AssertOK().
			AssertJSONPath("data.0.object", "conversation_attribute").
			AssertJSONPath("data.0.filled_by", "teammates").
			AssertJSONPath("data.0.required_to_close", true).
			AssertJSONPath("data.0.team_ids.0", team.Data.ID)

		d.owner.PatchJSON("/api/v1/conversation-attributes/"+id, testkit.JSON{
			"filled_by": "both", "required_to_close": false, "team_ids": []string{},
		}).AssertOK().
			AssertJSONPath("data.filled_by", "both").
			AssertJSONPath("data.required_to_close", false).
			AssertJSONPath("data.team_ids.#", 0)

		d.owner.GetJSON("/api/v1/ai/conversation-attributes").AssertOK().
			AssertHeader("Deprecation", "@1795651200").
			AssertJSONPath("data.0.key", "issue")
	})
}
