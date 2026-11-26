package intercom

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
)

/*
Golden tests: Intercom payloads in testdata/in, scrubbed of customer
content, translated into the import format and compared with
testdata/golden. Run with -update to rewrite the golden files after an
intended change, then read the diff.
*/
var update = flag.Bool("update", false, "rewrite the golden files")

func read(t *testing.T, name string, v any) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "in", name))
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

/*
golden compares the records, with the files they name, with the golden
file.
*/
func golden(t *testing.T, name string, out any) {
	t.Helper()

	got, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	got = append(got, '\n')
	path := filepath.Join("testdata", "golden", name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v: run go test -update to write it", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file:\n%s", name, got)
	}
}

/*
sent is a record as the engine sends it: the record, and the files it
uploads first.
*/
type sent struct {
	Kind   format.Kind    `json:"kind"`
	Record any            `json:"record"`
	Files  []*format.File `json:"files,omitempty"`
}

func asSent(recs ...format.Record) []sent {
	out := make([]sent, 0, len(recs))
	for _, r := range recs {
		out = append(out, sent{Kind: r.Kind, Record: r.Data, Files: r.Files})
	}

	return out
}

func knownSet(ids []string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}

	return m
}

func TestConversationGolden(t *testing.T) {
	var in struct {
		Conversation conversation `json:"conversation"`
		Known        []string     `json:"known_admins"`
	}

	read(t, "conversation.json", &in)

	rec, former := translateConversation(in.Conversation, in.Conversation.Parts.Parts, knownSet(in.Known))
	recs := []format.Record{}

	for _, f := range former {
		recs = append(recs, translateFormer(f))
	}

	golden(t, "conversation.json", asSent(append(recs, rec)...))

	conv := rec.Data.(*format.Conversation)

	for _, e := range conv.Entries {
		if e.Type == format.EntryEvent && e.Event.RawType == "" {
			t.Errorf("event %s lost its Intercom type", e.SourceID)
		}
	}

	if len(former) != 1 || former[0].ID != "502" {
		t.Errorf("former teammates %+v, want the note's author 502", former)
	}

	if len(rec.Files) != 2 {
		t.Errorf("files %d, want the source's and the note's", len(rec.Files))
	}
}

func TestEveryPartTypeIsKept(t *testing.T) {
	types := []string{
		"comment", "note", "assignment", "assign_and_reopen", "open", "close", "snoozed", "timer_unsnooze",
		"conversation_attribute_updated_by_admin", "conversation_attribute_updated_by_workflow", "operator_workflow_event",
		"custom_action_started", "ticket_state_updated_by_admin", "merged_primary_conversation", "unknown", "made_up_type",
	}

	tr := &convTranslation{contacts: map[string]bool{}, former: map[string]Former{}, seen: map[string]bool{}}

	for _, typ := range types {
		got := tr.entry(part{ID: ID("p-" + typ), PartType: typ, Body: "<p>x</p>", CreatedAt: 1, Author: author{Type: "admin", ID: "1"}})
		if len(got) == 0 {
			t.Errorf("%s was dropped", typ)
		}
	}
}

func TestArticleGolden(t *testing.T) {
	var a article
	read(t, "article.json", &a)

	rec := translateArticle(a, "hc-1", "en")
	golden(t, "article.json", asSent(rec))

	art := rec.Data.(*format.Article)
	if len(art.Translations) != 1 || art.Translations[0].Locale != "fr" {
		t.Fatalf("translations %+v, want only fr: en is the article itself", art.Translations)
	}

	if !strings.Contains(strings.Join(art.Blocks, ","), "button") || !strings.Contains(strings.Join(art.Blocks, ","), "video") {
		t.Errorf("blocks %v", art.Blocks)
	}
}

func TestMacroPlaceholders(t *testing.T) {
	var m macro
	read(t, "macro.json", &m)

	rec := translateMacro(m)
	golden(t, "macro.json", asSent(rec))

	r := rec.Data.(*format.SavedReply)
	if !strings.Contains(r.Body, `{{ contact.first_name | default: "there" }}`) || !strings.Contains(r.Body, "{{ company.name }}") ||
		!strings.Contains(r.Body, "{{ sender.first_name }}") {
		t.Errorf("body %q", r.Body)
	}

	if len(r.Unknown) != 1 || r.Unknown[0] != "user.custom_attributes.plan" {
		t.Errorf("unknown %v", r.Unknown)
	}
}

func TestScheduleGolden(t *testing.T) {
	var in struct {
		Schedule   schedule            `json:"schedule"`
		Exceptions []scheduleException `json:"exceptions"`
	}

	read(t, "schedule.json", &in)
	golden(t, "schedule.json", asSent(translateSchedule(in.Schedule, in.Exceptions, true)))
}

func TestTicketGolden(t *testing.T) {
	var in struct {
		Ticket ticket   `json:"ticket"`
		Known  []string `json:"known_admins"`
	}

	read(t, "ticket.json", &in)

	rec, _ := translateTicket(in.Ticket, knownSet(in.Known))
	golden(t, "ticket.json", asSent(rec))
}

func TestContactGolden(t *testing.T) {
	var c contact
	read(t, "contact.json", &c)

	rec := translateContact(c, refIDs(c.Tags.Data), refIDs(c.Companies.Data))
	golden(t, "contact.json", asSent(rec))

	got := rec.Data.(*format.Contact)
	if got.Name != "Ada <Lovelace>" || got.Email != "ada&co@example.com" {
		t.Errorf("name %q email %q: escapes must be undone", got.Name, got.Email)
	}
}

func TestWorkflowGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "in", "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}

	rec, err := translateWorkflow("12345", raw)
	if err != nil {
		t.Fatal(err)
	}

	wf := rec.Data.(*format.Workflow)
	if wf.Enabled {
		t.Error("a workflow must arrive disabled")
	}

	wf.Definition = nil
	golden(t, "workflow.json", asSent(rec))
}

func TestWorkflowID(t *testing.T) {
	for in, want := range map[string]string{
		"https://app.intercom.com/a/apps/abc123/automation/workflows/987654": "987654",
		"987654": "987654",
		"https://app.intercom.com/a/apps/abc123/automation/workflows/987654/edit": "987654",
	} {
		if got, ok := WorkflowID(in); !ok || got != want {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}

	if _, ok := WorkflowID("not a url"); ok {
		t.Error("accepted a non-url")
	}
}

func TestWeeklySplitsAtMidnight(t *testing.T) {
	got := weekly([]interval{{StartMinute: 1380, EndMinute: 1500}})
	if len(got) != 2 || got[0].Day != "monday" || got[0].End != "24:00" || got[1].Day != "tuesday" || got[1].End != "01:00" {
		t.Errorf("%+v", got)
	}
}
