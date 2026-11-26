package intercom

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/dimmihq/dimmi/cli/format"
)

/*
TestRealShapesDecode reads the payload shapes an inventory recorded from
a real Intercom workspace (testdata/shapes, every value scrubbed to its
type) into the reader's types. A field Intercom sends with another type
than the reader expects fails here, not halfway through a migration.
*/
func TestRealShapesDecode(t *testing.T) {
	targets := map[string]func() any{
		"admins.json":                  func() any { return &struct{ Admins []admin }{} },
		"teams.json":                   func() any { return &struct{ Teams []team }{} },
		"tags.json":                    func() any { return &list[tag]{} },
		"data_attributes_contact.json": func() any { return &list[dataAttribute]{} },
		"data_attributes_company.json": func() any { return &list[dataAttribute]{} },
		"conversation_attributes.json": func() any { return &list[conversationAttribute]{} },
		"companies_scroll.json":        func() any { return &list[company]{} },
		"ticket_types.json":            func() any { return &list[ticketType]{} },
		"help_centers.json":            func() any { return &list[helpCenter]{} },
		"collections.json":             func() any { return &list[collection]{} },
		"snippets.json":                func() any { return &list[snippet]{} },
		"macros.json":                  func() any { return &list[macro]{} },
		"office_hours.json":            func() any { return &list[schedule]{} },
		"internal_articles.json":       func() any { return &list[internalArticle]{} },
		"contacts.json":                func() any { return &list[contact]{} },
		"articles.json":                func() any { return &list[article]{} },
		"conversation.json":            func() any { return &conversation{} },
		"ticket.json":                  func() any { return &ticket{} },
	}

	files, _ := filepath.Glob(filepath.Join("testdata", "shapes", "*.json"))
	if len(files) == 0 {
		t.Fatal("no recorded shapes")
	}

	for _, f := range files {
		name := filepath.Base(f)

		target, ok := targets[name]
		if !ok {
			continue
		}

		raw, err := fs.ReadFile(os.DirFS(filepath.Join("testdata", "shapes")), name)
		if err != nil {
			t.Fatal(err)
		}

		if err := json.Unmarshal(raw, target()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

/*
TestRealWorkflowExport translates the workflow export Intercom gave for a
workflow made from its routing template: every step is listed, its
assignment step is one dimmi can run, and it arrives switched off.
*/
func TestRealWorkflowExport(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "shapes", "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}

	rec, err := translateWorkflow("73828290", raw)
	if err != nil {
		t.Fatal(err)
	}

	wf, ok := rec.Data.(*format.Workflow)
	if !ok || wf.Enabled {
		t.Fatalf("workflow %+v", rec.Data)
	}

	types := map[string]bool{}
	for _, s := range wf.Steps {
		types[s.SourceType] = s.Supported
	}

	for _, want := range []string{"conditional_branches", "mid_path_actions", "terminal", "assign-conversation"} {
		if supported, found := types[want]; !found || !supported {
			t.Errorf("step %s: found %v supported %v in %+v", want, found, supported, wf.Steps)
		}
	}
}
