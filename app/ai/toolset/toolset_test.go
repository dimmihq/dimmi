package toolset_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
)

func entries() []toolset.Entry {
	return []toolset.Entry{
		{Group: "tickets", Spec: contracts.ToolSpec{
			Name: "open_ticket", Description: "Open a ticket for work the team must follow up. Returns its id.",
			Schema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},` +
				`"priority":{"type":"string","enum":["low","high"]}},"required":["title"],` +
				`"additionalProperties":false}`),
		}},
		{Group: "orders", Reads: true, Spec: contracts.ToolSpec{
			Name: "action_lookup_shipment", Description: "Look up a shipment by tracking number. Read-only.",
			Schema: json.RawMessage(`{"type":"object","properties":{"tracking":{"type":"string"}},` +
				`"required":["tracking"],"additionalProperties":false}`),
		}},
		{Group: "orders", Spec: contracts.ToolSpec{
			Name: "action_void_label", Description: "Void an unused shipping label and refund it.",
			Schema: json.RawMessage(`{"type":"object","properties":{"label_id":{"type":"string"}},` +
				`"required":["label_id"]}`),
		}},
	}
}

func groups() []toolset.Group {
	return []toolset.Group{
		{Name: "tickets", Line: "open and follow tickets"}, {Name: "orders", Line: "shipments and labels"},
	}
}

func TestFixedSpecsNeverVary(t *testing.T) {
	a, b := toolset.Specs(), toolset.Specs()
	if len(a) != 2 || a[0].Name != toolset.LoadTool || a[1].Name != toolset.CallTool {
		t.Fatalf("specs = %+v", a)
	}
	for i := range a {
		if string(a[i].Schema) != string(b[i].Schema) || a[i].Description != b[i].Description {
			t.Fatal("specs differ between calls")
		}
		var v any
		if err := json.Unmarshal(a[i].Schema, &v); err != nil {
			t.Fatalf("schema of %s: %v", a[i].Name, err)
		}
	}
}

func TestPromptListsGroupsOneLineEach(t *testing.T) {
	p := toolset.New(entries(), groups(), nil).Prompt()
	if !strings.Contains(p, "- tickets: open and follow tickets (open_ticket)") {
		t.Fatalf("prompt = %q", p)
	}
	if !strings.Contains(p, "- orders: shipments and labels (action_lookup_shipment, action_void_label)") {
		t.Fatalf("prompt = %q", p)
	}
}

func TestCallNeedsLoadFirst(t *testing.T) {
	s := toolset.New(entries(), groups(), nil)
	_, err := s.Check(json.RawMessage(`{"name":"open_ticket","arguments":{"title":"x"}}`))
	if !errors.Is(err, toolset.ErrNotLoaded) || !strings.Contains(err.Error(), "load_tools") {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.Load(json.RawMessage(`{"names":["open_ticket"]}`)); err != nil {
		t.Fatal(err)
	}
	c, err := s.Check(json.RawMessage(`{"name":"open_ticket","arguments":{"title":"x"}}`))
	if err != nil || c.Name != "open_ticket" {
		t.Fatalf("call = %+v, %v", c, err)
	}
}

func TestArgumentValidationAnswersShortly(t *testing.T) {
	s := toolset.New(entries(), groups(), nil)
	_, _ = s.Load(json.RawMessage(`{"names":["open_ticket"]}`))
	cases := []string{
		`{"name":"open_ticket","arguments":{}}`,
		`{"name":"open_ticket","arguments":{"title":"x","priority":"urgent"}}`,
		`{"name":"open_ticket","arguments":{"title":"x","color":"red"}}`,
		`{"name":"open_ticket","arguments":{"title":5}}`,
	}
	for _, c := range cases {
		_, err := s.Check(json.RawMessage(c))
		if !errors.Is(err, toolset.ErrArguments) {
			t.Fatalf("%s: err = %v", c, err)
		}
		if len(err.Error()) > 300 || strings.Contains(err.Error(), "\n") {
			t.Fatalf("%s: refusal too long: %q", c, err)
		}
	}
	if got := s.Stats().ArgErrors; got != len(cases) {
		t.Fatalf("argument errors = %d", got)
	}
}

func TestUnknownToolIsCounted(t *testing.T) {
	s := toolset.New(entries(), groups(), nil)
	_, err := s.Check(json.RawMessage(`{"name":"refund_order"}`))
	if !errors.Is(err, toolset.ErrUnknownTool) || !strings.Contains(err.Error(), "tickets, orders") {
		t.Fatalf("err = %v", err)
	}
	out, err := s.Load(json.RawMessage(`{"names":["open_ticket","nope"]}`))
	if err != nil || !strings.Contains(out, `"not_found":["nope"]`) {
		t.Fatalf("load = %s, %v", out, err)
	}
	if nf := s.Stats().NotFound; len(nf) != 2 {
		t.Fatalf("not found = %v", nf)
	}
}

func TestLoadGroupListsTools(t *testing.T) {
	s := toolset.New(entries(), groups(), nil)
	out, err := s.Load(json.RawMessage(`{"group":"orders"}`))
	if err != nil || !strings.Contains(out, "action_void_label: Void an unused shipping label and refund it.") {
		t.Fatalf("index = %q, %v", out, err)
	}
	if len(s.Loaded()) != 0 {
		t.Fatal("listing a group loads nothing")
	}
}

func TestReadToolGainsFieldsAndReloadsCount(t *testing.T) {
	s := toolset.New(entries(), groups(), []string{"action_lookup_shipment"})
	out, err := s.Load(json.RawMessage(`{"names":["action_lookup_shipment","open_ticket"]}`))
	if err != nil || !strings.Contains(out, `"fields"`) {
		t.Fatalf("load = %s, %v", out, err)
	}
	c, err := s.Check(json.RawMessage(`{"name":"action_lookup_shipment",` +
		`"arguments":{"tracking":"1Z","fields":["status"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(c.Arguments) != `{"tracking":"1Z"}` || len(c.Fields) != 1 || c.Fields[0] != "status" {
		t.Fatalf("call = %s %v", c.Arguments, c.Fields)
	}
	st := s.Stats()
	if st.Loads != 2 || st.Reloads != 1 || st.Calls != 1 || st.Used[0] != "action_lookup_shipment" {
		t.Fatalf("stats = %+v", st)
	}
}

func TestShapeProjectsCapsAndCuts(t *testing.T) {
	s := toolset.New(entries(), groups(), nil).WithLimits(toolset.Limits{Items: 2, Text: 10})
	in := `{"data":[{"id":1,"title":"a","body":"x"},{"id":2,"title":"b","body":"y"},{"id":3,"title":"c"}]}`
	var got struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(s.Shape(in, []string{"title"})), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 3 || got.Data[2]["more"] != float64(1) {
		t.Fatalf("data = %+v", got.Data)
	}
	if _, ok := got.Data[0]["body"]; ok || got.Data[0]["id"] != float64(1) {
		t.Fatalf("item = %+v", got.Data[0])
	}

	cut := s.Shape(`{"text":"abcdefghijklmnopqrstuvwxyz"}`, nil)
	if !strings.Contains(cut, "abcdefghij …[cut: 16 more characters") || !strings.Contains(cut, `read_rest`) {
		t.Fatalf("cut = %s", cut)
	}
	rest, err := s.Rest(json.RawMessage(`{"ref":1}`))
	if err != nil || !strings.HasPrefix(rest, "klmnopqrst") {
		t.Fatalf("rest = %q, %v", rest, err)
	}
	if _, err := s.Rest(json.RawMessage(`{"ref":9}`)); !errors.Is(err, toolset.ErrArguments) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadRestNeedsNoLoad(t *testing.T) {
	s := toolset.New(entries(), groups(), nil)
	c, err := s.Check(json.RawMessage(`{"name":"read_rest","arguments":{"ref":1}}`))
	if err != nil || c.Name != toolset.ReadRest {
		t.Fatalf("call = %+v, %v", c, err)
	}
}

func TestKeywordsRankTheMatchingTool(t *testing.T) {
	names, err := toolset.Keywords{}.Rank(context.Background(), "Can you void the label I bought twice?", entries(), 20)
	if err != nil || len(names) == 0 || names[0] != "action_void_label" {
		t.Fatalf("ranked = %v, %v", names, err)
	}
	names, _ = toolset.Keywords{}.Rank(context.Background(), "where is shipment 1Z999", entries(), 20)
	if len(names) == 0 || names[0] != "action_lookup_shipment" {
		t.Fatalf("ranked = %v", names)
	}
}

func TestVectorsEmbedEachToolOnce(t *testing.T) {
	calls := 0
	embed := func(_ context.Context, texts []string) ([][]float32, error) {
		calls++
		out := make([][]float32, len(texts))
		for i, text := range texts {
			if strings.Contains(text, "Void") || strings.Contains(text, "refund my label") {
				out[i] = []float32{1, 0}
			} else {
				out[i] = []float32{0, 1}
			}
		}
		return out, nil
	}
	v := &toolset.Vectors{Model: "m", Embed: embed}
	for range 2 {
		names, err := v.Rank(context.Background(), "refund my label", entries(), 1)
		if err != nil || len(names) != 1 || names[0] != "action_void_label" {
			t.Fatalf("ranked = %v, %v", names, err)
		}
	}
	v.Query = []float32{1, 0}
	if _, err := v.Rank(context.Background(), "anything", entries(), 1); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("embed calls = %d, want tools once and the query once", calls)
	}
}

type decider struct{ pick string }

func (d decider) Decide(_ context.Context, req contracts.DecisionRequest) (*contracts.DecisionResponse, error) {
	q := req.Questions["tool"]
	p := map[string]float64{}
	for name := range q.Choices {
		p[name] = 0.1
	}
	p[d.pick] = 0.9
	return &contracts.DecisionResponse{Answers: map[string]contracts.DecisionAnswer{"tool": {Probabilities: p}}}, nil
}

func TestDecidedReordersAndIsOptional(t *testing.T) {
	msg := "void the shipment label"
	base, _ := toolset.Keywords{}.Rank(context.Background(), msg, entries(), 20)
	same, _ := toolset.Decided{Base: toolset.Keywords{}}.Rank(context.Background(), msg, entries(), 20)
	if strings.Join(base, ",") != strings.Join(same, ",") {
		t.Fatalf("without a decider %v != %v", same, base)
	}
	last := base[len(base)-1]
	got, _ := toolset.Decided{Base: toolset.Keywords{}, Decider: decider{pick: last}}.Rank(
		context.Background(), msg, entries(), 20)
	if got[0] != last {
		t.Fatalf("decided = %v", got)
	}
}

func TestNoteAndEarlier(t *testing.T) {
	if toolset.Note(nil, nil) != "" {
		t.Fatal("empty note")
	}
	n := toolset.Note([]string{"open_ticket"}, []string{"action_void_label"})
	if !strings.Contains(n, "load again before use: open_ticket.") ||
		!strings.Contains(n, "Likely tools for this message: action_void_label.") {
		t.Fatalf("note = %q", n)
	}
	got := toolset.Earlier([]json.RawMessage{
		json.RawMessage(`{"loaded":["b","a"]}`), json.RawMessage(`{}`),
		json.RawMessage(`{"loaded":["a","gone"]}`),
	}, func(n string) bool { return n != "gone" })
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("loaded = %v", got)
	}
}

func TestInlineThreshold(t *testing.T) {
	small := []contracts.ToolSpec{{Name: "a", Description: "short", Schema: json.RawMessage(`{}`)}}
	if !toolset.Inline(small) {
		t.Fatal("a small action stays inline")
	}
	big := make([]contracts.ToolSpec, 0, 10)
	for range 10 {
		big = append(big, contracts.ToolSpec{Name: "b", Description: strings.Repeat("x", 900)})
	}
	if toolset.Inline(big) || toolset.Inline(nil) {
		t.Fatal("many large actions, or none, are not inline")
	}
}

func TestShapeKeepsTheUntrustedWrapper(t *testing.T) {
	s := toolset.New(entries(), groups(), nil).WithLimits(toolset.Limits{Items: 5, Text: 10})
	out := s.Shape(`<action_result action="x">abcdefghijklmnopqrstuvwxyz</action_result>`, nil)
	if !strings.HasPrefix(out, `<action_result action="x">abcdefghij …[cut`) ||
		!strings.HasSuffix(out, "</action_result>") {
		t.Fatalf("shaped = %q", out)
	}
}

func TestLoadingAFixedToolSaysItIsOffered(t *testing.T) {
	s := toolset.New(entries(), groups(), nil).WithFixed([]string{"search_knowledge"})
	out, err := s.Load(json.RawMessage(`{"names":["search_knowledge"]}`))
	if err != nil || !strings.Contains(out, `"already_offered":["search_knowledge"]`) {
		t.Fatalf("load = %s, %v", out, err)
	}
	if len(s.Stats().NotFound) != 0 {
		t.Fatal("a fixed tool is not counted as not found")
	}
}

func TestCompactWritesLinesAndShortIDs(t *testing.T) {
	s := toolset.New(entries(), groups(), nil).WithCompact()
	id := "conv_01jcz7m8q4n5p6r7s8t9v0w1x2"
	out := s.Compact(`{"data":[{"id":"` + id + `","title":"Refund","note":null,"tags":[]},{"id":"` + id + `"}]}`)
	if strings.Contains(out, id) || strings.Contains(out, "null") || strings.Contains(out, "{") {
		t.Fatalf("compact = %q", out)
	}
	if !strings.Contains(out, "- id: c1; title: Refund") {
		t.Fatalf("compact = %q", out)
	}
	back, err := s.Expand(json.RawMessage(`{"conversation_id":"c1","tags":["c1"]}`))
	if err != nil || !strings.Contains(string(back), id) {
		t.Fatalf("expand = %s, %v", back, err)
	}
	_, err = s.Expand(json.RawMessage(`{"conversation_id":"c9"}`))
	if !errors.Is(err, toolset.ErrArguments) || !strings.Contains(err.Error(), "look the record up again") {
		t.Fatalf("err = %v", err)
	}
	plain := toolset.New(entries(), groups(), nil)
	if plain.Compact(`{"a":1}`) != `{"a":1}` {
		t.Fatal("compact is off unless asked for")
	}
}
