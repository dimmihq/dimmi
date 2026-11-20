/*
|--------------------------------------------------------------------------
| Writing Templates
|--------------------------------------------------------------------------
|
| A template a teammate writes in dimmi goes to Meta for review: a text
| header or none, a body, a footer, and buttons that need no values when
| sent (quick replies, a fixed link, a phone number). Variables are
| numbered, {{1}} onwards, each with the sample Meta reviews it with. An
| approved, rejected or paused template can be edited, which sends it to
| review again; its name and language stay. Deleting removes every
| language of the name at Meta, so dimmi deletes by the template's id.
|
*/

package whatsapp

import (
	"context"
	"net/url"
	"strings"
)

/*
Button types a written template may have.
*/
const (
	ButtonQuickReply = "QUICK_REPLY"
	ButtonURL        = "URL"
	ButtonPhone      = "PHONE_NUMBER"
)

/*
Draft is a template as a teammate wrote it. Samples hold a value for
each variable, keyed as the composer keys them: "header.1", "body.2".
*/
type Draft struct {
	Name     string
	Language string
	Category string
	Header   string
	Body     string
	Footer   string
	Buttons  []Action
	Samples  map[string]string
}

/*
Written is what Meta says of a template it took for review.
*/
type Written struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Category string `json:"category"`
}

/*
CreateTemplate sends the draft to Meta for review on the account.
*/
func (g *Graph) CreateTemplate(ctx context.Context, accountID string, d Draft) (Written, error) {
	body := map[string]any{
		"name": d.Name, "language": d.Language, "category": strings.ToUpper(d.Category),
		"parameter_format": "positional", "components": d.components(),
	}

	var out Written

	err := g.Post(ctx, accountID+"/message_templates", body, &out)

	return out, err
}

/*
EditTemplate sends the template's new content to Meta, which reviews it
again.
*/
func (g *Graph) EditTemplate(ctx context.Context, templateID string, d Draft) error {
	body := map[string]any{"components": d.components()}
	if d.Category != "" {
		body["category"] = strings.ToUpper(d.Category)
	}

	return g.Post(ctx, templateID, body, nil)
}

/*
DeleteTemplate removes the one template with the id and name at Meta.
*/
func (g *Graph) DeleteTemplate(ctx context.Context, accountID, templateID, name string) error {
	query := url.Values{"name": {name}, "hsm_id": {templateID}}

	return g.Delete(ctx, accountID+"/message_templates?"+query.Encode())
}

/*
components writes the draft as Meta's template parts.
*/
func (d Draft) components() []map[string]any {
	var out []map[string]any

	if d.Header != "" {
		header := map[string]any{"type": "HEADER", "format": "TEXT", "text": d.Header}
		if samples := d.samples("header", d.Header); len(samples) > 0 {
			header["example"] = map[string]any{"header_text": samples}
		}

		out = append(out, header)
	}

	body := map[string]any{"type": "BODY", "text": d.Body}
	if samples := d.samples("body", d.Body); len(samples) > 0 {
		body["example"] = map[string]any{"body_text": [][]string{samples}}
	}

	out = append(out, body)

	if d.Footer != "" {
		out = append(out, map[string]any{"type": "FOOTER", "text": d.Footer})
	}

	if len(d.Buttons) > 0 {
		out = append(out, map[string]any{"type": "BUTTONS", "buttons": d.Buttons})
	}

	return out
}

/*
samples lists the part's sample values in the order of its variables.
*/
func (d Draft) samples(part, text string) []string {
	var out []string

	for _, key := range Placeholders(text) {
		out = append(out, d.Samples[part+"."+key])
	}

	return out
}

/*
Placeholders lists the text's variables in order, each once.
*/
func Placeholders(text string) []string {
	var out []string

	seen := map[string]bool{}
	for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}

	return out
}
