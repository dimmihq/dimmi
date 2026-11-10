/*
|--------------------------------------------------------------------------
| Writing Each Message
|--------------------------------------------------------------------------
|
| A broadcast is written once, in Markdown with variables, and filled in
| for each recipient: their first name, their company, the sender. A
| missing detail uses the fallback the writer gave, never a blank where a
| name should be. Email turns the Markdown into HTML with every link
| counted on its way through, a pixel that estimates opens, and a footer
| with the one-click way out; the Messenger and Slack get the Markdown.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"html"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/models"
)

/*
variable finds one Liquid-style variable with its optional fallback.
*/
var variable = regexp.MustCompile(`\{\{\s*([a-z_]+(?:\.[a-z_0-9]+)*)\s*(?:\|\s*default:\s*"([^"]*)"\s*)?\}\}`)

/*
Rendered is one recipient's message: the filled subject and preview,
the Markdown the Messenger and Slack show, and the email's text and
HTML.
*/
type Rendered struct {
	Subject    string
	Preview    string
	Markdown   string
	Text       string
	HTML       string
	SenderName string
}

/*
render writes the broadcast for one recipient.
*/
func (b *Broadcasts) render(ctx context.Context, q *models.Queries, d Delivery) (Rendered, error) {
	vars, err := b.variables(ctx, q, d)
	if err != nil {
		return Rendered{}, err
	}

	row := d.Broadcast
	out := Rendered{
		Subject: fill(row.Subject, vars), Preview: fill(row.Preview, vars), Markdown: fill(row.Body, vars),
		SenderName: senderName(vars),
	}

	if row.Channel != ChannelEmail {
		return out, nil
	}

	links := []string{}
	r := markdown.New(markdown.Options{GFM: true, HardWraps: true, RewriteLink: func(l markdown.Link) markdown.Link {
		if l.Image || !row.TrackClicks || !trackable(l.URL) {
			return l
		}

		links = append(links, l.URL)
		l.URL = b.Links.Click(d.Recipient, len(links))

		return l
	}})

	page, err := r.HTML([]byte(out.Markdown))
	if err != nil {
		return out, wrap("render broadcast", err)
	}

	if err := b.keepLinks(ctx, q, row, links); err != nil {
		return out, err
	}

	footer := b.footer(d, vars["workspace.name"])
	out.HTML = preheader(out.Preview) + page.HTML + footer.html + openPixel(b.Links.Open(d.Recipient))
	out.Text = markdown.PlainText([]byte(out.Markdown)) + "\n\n" + footer.text

	return out, nil
}

/*
keepLinks records the links the message counts, in order, so a click
can say which one it was.
*/
func (b *Broadcasts) keepLinks(ctx context.Context, q *models.Queries, row models.Broadcast, links []string) error {
	for i, url := range links {
		err := q.InsertBroadcastLink(ctx, models.InsertBroadcastLinkParams{
			WorkspaceID: row.WorkspaceID, BroadcastID: row.ID, N: int32(i + 1), Url: url,
		})
		if err != nil {
			return wrap("keep broadcast link", err)
		}
	}

	return nil
}

/*
footer is what closes every broadcast email: who sent it and how to
stop getting them.
*/
type footer struct {
	text string
	html string
}

/*
footer writes the email's footer for the recipient.
*/
func (b *Broadcasts) footer(d Delivery, workspace string) footer {
	link := b.Links.Unsubscribe(d.Recipient)
	why := "You're getting this because you're a customer of " + workspace + "."

	return footer{
		text: "--\n" + why + "\nUnsubscribe or choose what you get: " + link,
		html: `<hr style="border:none;border-top:1px solid #e5e5e5;margin:32px 0 16px">` +
			`<p style="color:#737373;font-size:12px;line-height:18px;margin:0">` + html.EscapeString(why) +
			` <a href="` + html.EscapeString(link) + `" style="color:#737373">` +
			`Unsubscribe or choose what you get</a>.</p>`,
	}
}

/*
variables are the values a recipient's message fills in.
*/
func (b *Broadcasts) variables(ctx context.Context, q *models.Queries, d Delivery) (map[string]string, error) {
	ws := d.Broadcast.WorkspaceID
	vars := map[string]string{}

	workspace, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return nil, wrap("find workspace", err)
	}

	vars["workspace.name"] = workspace.Name

	if d.Broadcast.SenderMemberID != nil {
		sender, err := q.FindBroadcastSender(ctx, models.FindBroadcastSenderParams{
			WorkspaceID: ws, ID: *d.Broadcast.SenderMemberID,
		})
		if err == nil {
			vars["sender.name"], vars["sender.first_name"] = sender.Name, firstName(sender.Name)
		}
	}

	if d.Contact != nil {
		contactVars(vars, *d.Contact)
	}

	companyID := d.Recipient.CompanyID
	if companyID != nil {
		company, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: ws, ID: *companyID})
		if err == nil {
			vars["company.name"] = company.Name
			attributes(vars, "company.", company.CustomAttributes)
		}
	}

	return vars, nil
}

/*
contactVars fills in the contact's own details and custom attributes.
*/
func contactVars(vars map[string]string, c models.Contact) {
	attributes(vars, "contact.", c.CustomAttributes)

	if c.Name != nil {
		vars["contact.name"], vars["contact.first_name"] = *c.Name, firstName(*c.Name)
	}

	if c.Email != nil {
		vars["contact.email"] = *c.Email
	}
}

/*
attributes adds custom attributes that read as text under the prefix.
*/
func attributes(vars map[string]string, prefix string, raw json.RawMessage) {
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return
	}

	for k, v := range values {
		switch v := v.(type) {
		case string:
			vars[prefix+k] = v
		case float64, bool:
			b, _ := json.Marshal(v)
			vars[prefix+k] = string(b)
		}
	}
}

/*
fill replaces every variable with its value, or its fallback when the
recipient has none.
*/
func fill(text string, vars map[string]string) string {
	return variable.ReplaceAllStringFunc(text, func(m string) string {
		parts := variable.FindStringSubmatch(m)
		if v := strings.TrimSpace(vars[parts[1]]); v != "" {
			return v
		}

		return parts[2]
	})
}

/*
senderName is the name a message comes from: the teammate, and their
workspace after them, or the workspace alone.
*/
func senderName(vars map[string]string) string {
	if vars["sender.name"] == "" {
		return vars["workspace.name"]
	}

	return vars["sender.name"] + " from " + vars["workspace.name"]
}

/*
firstName is the first word of a name.
*/
func firstName(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

/*
trackable reports whether a link can be counted through a redirect.
*/
func trackable(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

/*
preheader is the hidden line inbox lists show after the subject.
*/
func preheader(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}

	return `<div style="display:none;max-height:0;overflow:hidden;opacity:0">` + html.EscapeString(text) + `</div>`
}

/*
openPixel is the image that tells the broadcast an email was opened.
*/
func openPixel(url string) string {
	return `<img src="` + html.EscapeString(url) + `" width="1" height="1" alt="" style="display:block;border:0">`
}
