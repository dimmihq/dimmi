package liquid

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
)

func render(t *testing.T, src string, globals map[string]any) string {
	t.Helper()
	out, err := tryRender(src, globals, Limits{})
	if err != nil {
		t.Fatalf("render %q: %v", src, err)
	}
	return out
}

func tryRender(src string, globals map[string]any, limits Limits) (string, error) {
	set, errs := New().ParseSet(map[string]string{"page": src, "partials/item": "<{{ item }}>"},
		func(name string) string { return "partials/" + name }, nil)
	if len(errs) > 0 {
		return "", errs[0]
	}
	var b strings.Builder
	_, err := set.Render(context.Background(), &b, "page", Options{Globals: globals, Limits: limits})
	return b.String(), err
}

func TestOutputAndFilters(t *testing.T) {
	cases := map[string]string{
		`{{ "hi" | upcase }}`:                               "HI",
		`{{ name }}`:                                        "Ada &lt;b&gt;",
		`{{ body }}`:                                        "<p>ok</p>",
		`{{ body | strip_html }}`:                           "ok",
		`{{ name | raw }}`:                                  "Ada <b>",
		`{{ name | escape }}`:                               "Ada &lt;b&gt;",
		`{{ 3 | plus: 4 }}`:                                 "7",
		`{{ 10 | divided_by: 4 }}`:                          "2",
		`{{ 10.0 | divided_by: 4 }}`:                        "2.5",
		`{{ list | join: ", " }}`:                           "a, b, c",
		`{{ list.size }} {{ list.first }} {{ list[1] }}`:    "3 a b",
		`{{ list | reverse | first }}`:                      "c",
		`{{ missing | default: "none" }}`:                   "none",
		`{{ "a,b" | split: "," | last }}`:                   "b",
		`{{ people | map: "name" | join: "&" }}`:            "Grace&amp;Ada",
		`{{ people | where: "admin", true | size }}`:        "1",
		`{{ "Hello world" | truncate: 8 }}`:                 "Hello...",
		`{{ when | date: "%B %-d, %Y" }}`:                   "October 8, 2026",
		`{{ "x" | t: count: 2 }}`:                           "x:2",
		`{{- "  a  " -}}`:                                   "  a  ",
		"a  {{- 'b' -}}  c":                                 "abc",
		`{% raw %}{{ not parsed }}{% endraw %}`:             "{{ not parsed }}",
		`{% comment %}{{ x {% endcomment %}done`:            "done",
		`{% # a note %}ok`:                                  "ok",
		`{% echo "e" | upcase %}`:                           "E",
		`{{ map.key }}-{{ map["key"] }}-{{ map.size }}`:     "v-v-1",
		`{{ list | json }}`:                                 `[&#34;a&#34;,&#34;b&#34;,&#34;c&#34;]`,
		`{{ list | json | raw }}`:                           `["a","b","c"]`,
		`{{ "5" | plus: 1 }}`:                               "6",
		`{{ 1.5 | round }} {{ 2.345 | round: 2 }}`:          "2 2.35",
		`{{ "b,a,c" | split: "," | sort | join: "" }}`:      "abc",
		`{{ people | sort: "name" | map: "name" | first }}`: "Ada",
	}
	e := New()
	e.Filter("t", func(c *Call) (any, error) { return toString(c.Input) + ":" + toString(c.Named["count"]), nil })
	for src, want := range cases {
		set, errs := e.ParseSet(map[string]string{"p": src}, nil, nil)
		if len(errs) > 0 {
			t.Fatalf("parse %q: %v", src, errs[0])
		}
		var b strings.Builder
		_, err := set.Render(context.Background(), &b, "p", Options{Globals: globals(t)})
		if err != nil {
			t.Fatalf("render %q: %v", src, err)
		}
		if b.String() != want {
			t.Errorf("%s = %q, want %q", src, b.String(), want)
		}
	}
}

func globals(t *testing.T) map[string]any {
	t.Helper()
	g, ok := Value(map[string]any{
		"name": "Ada <b>",
		"body": HTML("<p>ok</p>"),
		"list": []string{"a", "b", "c"},
		"map":  map[string]string{"key": "v"},
		"when": time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC),
		"people": []map[string]any{
			{"name": "Grace", "admin": false},
			{"name": "Ada", "admin": true},
		},
	}).(map[string]any)
	if !ok {
		t.Fatal("globals are not a map")
	}
	return g
}

func TestTags(t *testing.T) {
	cases := []struct{ src, want string }{
		{`{% if a %}yes{% else %}no{% endif %}`, "yes"},
		{`{% if missing %}yes{% elsif a %}elsif{% endif %}`, "elsif"},
		{`{% unless missing %}u{% endunless %}`, "u"},
		{`{% if "" %}empty strings are true{% endif %}`, "empty strings are true"},
		{`{% if s == empty %}e{% endif %}{% if s != blank %}b{% endif %}`, "e"},
		{`{% if n > 2 and n < 5 %}in{% endif %}`, "in"},
		{`{% if false or n == 3 %}or{% endif %}`, "or"},
		{`{% if list contains "b" %}has{% endif %}`, "has"},
		{`{% if "abc" contains "bc" %}sub{% endif %}`, "sub"},
		{`{% case n %}{% when 1, 2 %}low{% when 3 or 4 %}mid{% else %}hi{% endcase %}`, "mid"},
		{`{% for x in list %}{{ forloop.index }}{{ x }}` +
			`{% unless forloop.last %},{% endunless %}{% endfor %}`, "1a,2b,3c"},
		{`{% for x in list reversed limit: 2 %}{{ x }}{% endfor %}`, "ba"},
		{`{% for x in list offset: 1 %}{{ x }}{% endfor %}`, "bc"},
		{`{% for x in none %}x{% else %}empty{% endfor %}`, "empty"},
		{`{% for i in (1..5) %}{% if i == 4 %}{% break %}{% endif %}` +
			`{% if i == 2 %}{% continue %}{% endif %}{{ i }}{% endfor %}`, "13"},
		{`{% assign y = "v" | upcase %}{{ y }}`, "V"},
		{`{% capture c %}<{{ n }}>{% endcapture %}{{ c }}`, "<3>"},
		{`{% render 'item' with n %}`, "<3>"},
		{`{% render 'item', item: "z" %}`, "<z>"},
		{`{% render 'item' for list %}`, "<a><b><c>"},
		{`{% assign item = "parent" %}{% render 'item' %}`, "<>"},
		{`{% for x in list %}{% for y in list %}{{ forloop.parentloop.index }}{% endfor %}{% endfor %}`, "111222333"},
	}
	g := map[string]any{"a": true, "n": 3, "s": "", "list": []any{"a", "b", "c"}}
	for _, c := range cases {
		if got := render(t, c.src, g); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestParseErrorsNameTheLine(t *testing.T) {
	cases := map[string]string{
		"a\n{% if x %}":           "page:2: missing {% endif %}",
		"{{ x | nope }}":          `unknown filter "nope"`,
		"\n\n{% frobnicate %}":    `page:3: unknown tag "frobnicate"`,
		"{{ x ":                   "missing }}",
		"{% for x %}{% endfor %}": "for needs the form",
	}
	for src, want := range cases {
		_, err := tryRender(src, nil, Limits{})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %v, want %q", src, err, want)
		}
	}
}

func TestLimitsStopRunawayTemplates(t *testing.T) {
	loops := `{% for a in (1..1000) %}{% for b in (1..1000) %}{% for c in (1..1000) %}` +
		`x{% endfor %}{% endfor %}{% endfor %}`
	_, err := tryRender(loops, nil, Limits{Steps: 100000, Time: time.Second, Output: 1 << 20, Depth: 4})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("nested loops: %v, want the limit", err)
	}

	_, err = tryRender(`{% for i in (1..1000000000) %}{% endfor %}`, nil, Limits{})
	if err == nil {
		t.Fatal("a huge range rendered")
	}

	_, err = tryRender(`{% for a in (1..10000) %}{{ "xxxxxxxxxx" }}{% endfor %}`, nil,
		Limits{Steps: 1 << 30, Time: time.Second, Output: 1000, Depth: 4})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("output: %v, want the limit", err)
	}

	set, _ := New().ParseSet(map[string]string{"page": "{% render 'self' %}", "self": "{% render 'self' %}"}, nil, nil)
	_, err = set.Render(context.Background(), &strings.Builder{}, "page", Options{})
	if err == nil || !strings.Contains(err.Error(), "nest deeper") {
		t.Fatalf("recursion: %v", err)
	}
}

func TestFallbackSetAndLayoutAndAssigns(t *testing.T) {
	e := New()
	base, _ := e.ParseSet(map[string]string{"partials/hi": "base hi", "partials/bye": "base bye"},
		func(n string) string { return "partials/" + n }, nil)
	custom, _ := e.ParseSet(map[string]string{
		"page":        "{% layout none %}{% assign title = 'T' %}{% render 'hi' %} {% render 'bye' %}",
		"partials/hi": "custom hi",
	}, func(n string) string { return "partials/" + n }, base)
	var b strings.Builder
	res, err := custom.Render(context.Background(), &b, "page", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != "custom hi base bye" || !res.LayoutSet || res.Layout != "" || res.Assigned["title"] != "T" {
		t.Fatalf("got %q %+v", b.String(), res)
	}
}

func TestValueReadsStructs(t *testing.T) {
	type person struct {
		Name      string
		SignInURL string
		Secret    string `liquid:"-"`
		Nick      string `liquid:"handle"`
		hidden    string
	}
	ada := person{Name: "Ada", SignInURL: "/in", Secret: "s", Nick: "a", hidden: "h"}
	v := asMap(t, Value(struct {
		People []person
		Lead   *person
		None   *person
	}{People: []person{ada}, Lead: &person{Name: "G"}}))
	people, ok := v["people"].([]any)
	if !ok || len(people) != 1 {
		t.Fatalf("people: %#v", v["people"])
	}
	p := asMap(t, people[0])
	if p["name"] != "Ada" || p["sign_in_url"] != "/in" || p["handle"] != "a" || p["secret"] != nil || len(p) != 3 {
		t.Fatalf("person: %#v", p)
	}
	if asMap(t, v["lead"])["name"] != "G" || v["none"] != nil {
		t.Fatalf("pointers: %#v", v)
	}
	if SnakeCase("ServerURL") != "server_url" || SnakeCase("HTMLBody") != "html_body" ||
		SnakeCase("AskAI") != "ask_ai" {
		t.Fatal(SnakeCase("ServerURL"), SnakeCase("HTMLBody"), SnakeCase("AskAI"))
	}
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not a map: %#v", v)
	}
	return m
}

func TestDateNowReadsTheClock(t *testing.T) {
	set, errs := New().ParseSet(map[string]string{"p": `{{ "now" | date: "%Y-%m-%d %H:%M" }}`}, nil, nil)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	frozen := clock.NewFake(time.Date(2026, 3, 4, 5, 6, 0, 0, time.UTC))
	var b strings.Builder
	if _, err := set.Render(context.Background(), &b, "p", Options{Clock: frozen}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "2026-03-04 05:06" {
		t.Fatalf("now = %q", b.String())
	}
}

func TestRenderTemplateWithLocals(t *testing.T) {
	e := New()
	set, _ := e.ParseSet(nil, nil, nil)
	layout, err := e.Parse("layout", "[{{ title }}]{% assign seen = true %}")
	if err != nil {
		t.Fatal(err)
	}
	locals := map[string]any{"title": "T"}
	var b strings.Builder
	res, err := set.RenderTemplate(context.Background(), &b, layout, Options{Locals: locals})
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != "[T]" || locals["seen"] != true || res.Assigned["seen"] != true {
		t.Fatalf("got %q %+v", b.String(), res)
	}
}

func TestEncodeDecodeKeepsHTMLAndTimes(t *testing.T) {
	when := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	in := map[string]any{"body": HTML("<p>x</p>"), "at": when, "n": 3, "list": []any{"a", HTML("<b>")}}
	raw, err := json.Marshal(Encode(in))
	if err != nil {
		t.Fatal(err)
	}
	var back any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	out, ok := Decode(back).(map[string]any)
	at, isTime := out["at"].(time.Time)
	items, isList := out["list"].([]any)
	if !ok || !isTime || !isList || out["body"] != HTML("<p>x</p>") || out["n"] != 3 || !at.Equal(when) ||
		items[1] != HTML("<b>") {
		t.Fatalf("decoded %#v", out)
	}
}
