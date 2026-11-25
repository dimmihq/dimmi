package channels_test

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
serveTemplates answers the calls that write templates: creating one
takes it for review, unless its name is taken; editing T1 and deleting
one are recorded.
*/
func (f *fakeGraph) serveTemplates(w fhttp.ResponseWriter, call graphCall) bool {
	path := call.Path

	switch {
	case call.Method == fhttp.MethodPost && path == waAccount+"/message_templates":
		if call.Body["name"] == "taken" {
			w.WriteHeader(fhttp.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"code":100,"error_subcode":2388024,"message":"Invalid parameter",`+
				`"error_user_msg":"There is already content for this template in English (US)."}}`)

			return true
		}

		f.write("T9", map[string]any{
			"id": "T9", "name": call.Body["name"], "language": call.Body["language"], "status": "PENDING",
			"category": call.Body["category"], "components": call.Body["components"],
		})
		_, _ = io.WriteString(w, `{"id":"T9","status":"PENDING","category":"UTILITY"}`)

		return true
	case call.Method == fhttp.MethodPost && path == "T9":
		f.write("T9", map[string]any{
			"id": "T9", "name": "order_ready", "language": "en_US", "status": "PENDING", "category": "UTILITY",
			"components": call.Body["components"],
		})
		_, _ = io.WriteString(w, `{"success":true}`)

		return true
	case call.Method == fhttp.MethodDelete && path == waAccount+"/message_templates":
		f.mu.Lock()
		delete(f.written, "T9")
		f.mu.Unlock()
		_, _ = io.WriteString(w, `{"success":true}`)

		return true
	default:
		return false
	}
}

func (f *fakeGraph) write(id string, template map[string]any) {
	raw, _ := json.Marshal(template)

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.written == nil {
		f.written = map[string]string{}
	}

	f.written[id] = string(raw)
}

/*
writtenJSON returns the templates written through the fake, each led by
a comma, to follow the fixed ones in a listing.
*/
func (f *fakeGraph) writtenJSON() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	keys := make([]string, 0, len(f.written))
	for k := range f.written {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var out strings.Builder
	for _, k := range keys {
		out.WriteString("," + f.written[k])
	}

	return out.String()
}

func TestWhatsAppTemplateWriting(t *testing.T) {
	t.Parallel()

	draft := func() map[string]any {
		return map[string]any{
			"name": "order_ready", "language": "en_US", "category": "utility",
			"header": "Order {{1}}", "body": "Hi {{1}}, your order {{2}} is ready.", "footer": "Acme",
			"buttons": []any{
				map[string]any{"type": "url", "text": "Track", "url": "https://acme.test/track"},
				map[string]any{"type": "quick_reply", "text": "Thanks"},
			},
			"samples": map[string]any{"header.1": "#42", "body.1": "Ada", "body.2": "#42"},
			"bindings": map[string]any{
				"body.1": map[string]any{"key": "contact.first_name", "fallback": "there"},
				"body.2": map[string]any{"key": "custom.order_number"},
				"body.9": map[string]any{"key": "contact.email"},
			},
		}
	}

	t.Run("a new template goes to Meta for review with its samples and is listed waiting", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)

		res := wc.ActingAsOwner().PostJSON("/api/v1/channels/whatsapp/"+wc.publicID()+"/templates", draft()).
			AssertCreated()
		res.AssertJSONPath("data.name", "order_ready")
		res.AssertJSONPath("data.status", "pending")
		res.AssertJSONPath("data.content.samples", map[string]any{"header.1": "#42", "body.1": "Ada", "body.2": "#42"})
		res.AssertJSONPath("data.bindings", map[string]any{
			"body.1": map[string]any{"key": "contact.first_name", "fallback": "there"},
			"body.2": map[string]any{"key": "custom.order_number"},
		})

		var sent graphCall
		for _, c := range wc.graph.calls {
			if c.Method == fhttp.MethodPost && c.Path == waAccount+"/message_templates" {
				sent = c
			}
		}

		require.NotNil(t, sent.Body)
		assert.Equal(t, "UTILITY", sent.Body["category"])
		raw, _ := json.Marshal(sent.Body["components"])
		assert.Contains(t, string(raw), `"body_text":[["Ada","#42"]]`)
		assert.Contains(t, string(raw), `"header_text":["#42"]`)
		assert.Less(t, bytes.Index(raw, []byte("QUICK_REPLY")), bytes.Index(raw, []byte(`"URL"`)),
			"quick replies come first")
	})

	t.Run("what Meta would refuse at once is caught on its field", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		url := "/api/v1/channels/whatsapp/" + wc.publicID() + "/templates"

		cases := map[string]func(map[string]any){
			"name": func(d map[string]any) { d["name"] = "Order Ready" },
			"body": func(d map[string]any) { d["body"] = "{{1}}, your order is ready." },
			"samples.body.2": func(d map[string]any) {
				d["samples"] = map[string]any{"header.1": "#42", "body.1": "Ada"}
			},
			"footer": func(d map[string]any) { d["footer"] = "Ref {{1}}" },
			"buttons.1.url": func(d map[string]any) {
				d["buttons"] = []any{
					map[string]any{"type": "url", "text": "Track", "url": "http://x"},
					map[string]any{"type": "quick_reply", "text": "Thanks"},
				}
			},
		}

		for field, change := range cases {
			d := draft()
			change(d)
			res := wc.ActingAsOwner().PostJSON(url, d).AssertStatus(422)
			assert.Contains(t, string(res.Body), field, field)
		}
	})

	t.Run("Meta's refusal comes back in its words", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		d := draft()
		d["name"] = "taken"

		res := wc.ActingAsOwner().PostJSON("/api/v1/channels/whatsapp/"+wc.publicID()+"/templates", d).
			AssertStatus(422)
		assert.Contains(t, string(res.Body), "There is already content for this template")
	})

	t.Run("an edited template goes back to review and a deleted one is gone", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		base := "/api/v1/channels/whatsapp/" + wc.publicID() + "/templates"
		id := wc.ActingAsOwner().PostJSON(base, draft()).AssertCreated().String("data.id")

		d := draft()
		delete(d, "name")
		delete(d, "language")
		d["body"] = "Hi {{1}}, order {{2}} is ready to collect."
		wc.ActingAsOwner().PatchJSON(base+"/"+id, d).AssertOK().
			AssertJSONPath("data.content.body", "Hi {{1}}, order {{2}} is ready to collect.")

		wc.ActingAsOwner().Delete(base + "/" + id).AssertStatus(204)
		list := wc.ActingAsOwner().GetJSON(base).AssertOK()
		assert.NotContains(t, string(list.Body), "order_ready")
	})
}
