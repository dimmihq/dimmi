/*
|--------------------------------------------------------------------------
| Message Templates
|--------------------------------------------------------------------------
|
| Outside the 24-hour window only a template Meta approved may go out.
| A template is a header, a body, a footer and buttons, its header and
| body holding variables as {{1}} or {{name}}. The channel sends text
| headers and bodies with their variables filled in; a template with a
| media header, a carousel, or buttons that need values of their own is
| listed but not sent from the inbox yet. The thread shows the template
| as the customer reads it, its variables filled in.
|
*/

package whatsapp

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

/*
Template is one of an account's templates as Meta lists it.
*/
type Template struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Language        string      `json:"language"`
	Status          string      `json:"status"`
	Category        string      `json:"category"`
	ParameterFormat string      `json:"parameter_format"`
	RejectedReason  string      `json:"rejected_reason"`
	Components      []Component `json:"components"`
}

/*
Component is one part of a template.
*/
type Component struct {
	Type    string   `json:"type"`
	Format  string   `json:"format,omitempty"`
	Text    string   `json:"text,omitempty"`
	Buttons []Action `json:"buttons,omitempty"`
	Example *Example `json:"example,omitempty"`
}

/*
Example holds the sample values Meta reviewed a part's variables with.
*/
type Example struct {
	HeaderText []string   `json:"header_text,omitempty"`
	BodyText   [][]string `json:"body_text,omitempty"`
}

/*
Action is a button of a template.
*/
type Action struct {
	Type        string   `json:"type"`
	Text        string   `json:"text"`
	URL         string   `json:"url,omitempty"`
	PhoneNumber string   `json:"phone_number,omitempty"`
	Example     []string `json:"example,omitempty"`
}

/*
Variable is a value a template needs: the part it sits in and its key,
a number or a name.
*/
type Variable struct {
	Part string `json:"part"`
	Key  string `json:"key"`
}

/*
ListTemplates reads every template of the WhatsApp account.
*/
func (g *Graph) ListTemplates(ctx context.Context, accountID string) ([]Template, error) {
	query := url.Values{
		"fields": {"id,name,language,status,category,parameter_format,rejected_reason,components"},
		"limit":  {"100"},
	}

	var (
		out   []Template
		after string
	)

	for range 20 {
		if after != "" {
			query.Set("after", after)
		}

		var page struct {
			Data   []Template `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}

		if err := g.Get(ctx, accountID+"/message_templates", query, &page); err != nil {
			return nil, err
		}

		out = append(out, page.Data...)
		if page.Paging.Next == "" || page.Paging.Cursors.After == "" {
			break
		}

		after = page.Paging.Cursors.After
	}

	return out, nil
}

/*
Variables lists the values the template needs, header first, each once.
*/
func (t Template) Variables() []Variable {
	var out []Variable

	for _, part := range []string{"HEADER", "BODY"} {
		c, ok := t.component(part)
		if !ok || (part == "HEADER" && c.Format != "" && c.Format != "TEXT") {
			continue
		}

		seen := map[string]bool{}
		for _, m := range placeholder.FindAllStringSubmatch(c.Text, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, Variable{Part: strings.ToLower(part), Key: m[1]})
			}
		}
	}

	return out
}

/*
Sendable reports whether the inbox can send the template: approved,
with a text header or none, and buttons that need no values.
*/
func (t Template) Sendable() bool {
	if !strings.EqualFold(t.Status, "APPROVED") {
		return false
	}

	for _, c := range t.Components {
		if !partSendable(c) {
			return false
		}
	}

	return true
}

/*
partSendable reports whether the inbox can send one part of a template:
a header of text or none, buttons that need no values, and no carousel
or offer.
*/
func partSendable(c Component) bool {
	switch c.Type {
	case "HEADER":
		return c.Format == "" || c.Format == "TEXT"
	case "BUTTONS":
		for _, b := range c.Buttons {
			if !buttonSendable(b) {
				return false
			}
		}

		return true
	case "CAROUSEL", "LIMITED_TIME_OFFER":
		return false
	default:
		return true
	}
}

/*
buttonSendable reports whether a button goes out without a value of its
own: a quick reply, a call or a fixed link.
*/
func buttonSendable(b Action) bool {
	switch b.Type {
	case "OTP", "COPY_CODE", "CATALOG", "MPM", "FLOW":
		return false
	case "URL":
		return !placeholder.MatchString(b.URL)
	default:
		return true
	}
}

/*
component returns the template's part of the type.
*/
func (t Template) component(kind string) (Component, bool) {
	for _, c := range t.Components {
		if c.Type == kind {
			return c, true
		}
	}

	return Component{}, false
}

/*
Render returns the template as the customer reads it, its variables
filled in with the values, a missing one left as it is written.
*/
func (t Template) Render(values map[string]string) string {
	var parts []string

	for _, kind := range []string{"HEADER", "BODY", "FOOTER"} {
		c, ok := t.component(kind)
		if !ok || c.Text == "" {
			continue
		}

		text := fill(c.Text, strings.ToLower(kind), values)
		if kind == "HEADER" {
			text = "**" + text + "**"
		}

		if kind == "FOOTER" {
			text = "_" + text + "_"
		}

		parts = append(parts, text)
	}

	if c, ok := t.component("BUTTONS"); ok {
		labels := make([]string, 0, len(c.Buttons))
		for _, b := range c.Buttons {
			labels = append(labels, "["+b.Text+"]")
		}

		if len(labels) > 0 {
			parts = append(parts, strings.Join(labels, " "))
		}
	}

	return strings.Join(parts, "\n\n")
}

/*
fill puts each value in place of its variable in the part.
*/
func fill(text, part string, values map[string]string) string {
	return placeholder.ReplaceAllStringFunc(text, func(match string) string {
		key := placeholder.FindStringSubmatch(match)[1]
		if v, ok := values[part+"."+key]; ok && v != "" {
			return v
		}

		if v, ok := values[key]; ok && v != "" {
			return v
		}

		return match
	})
}
