/*
|--------------------------------------------------------------------------
| Jira Deliveries
|--------------------------------------------------------------------------
|
| An OAuth app registers its webhooks through the REST API, with a JQL
| filter and the events it wants; Jira lets them lapse after thirty
| days unless refreshed, which the sync does. A delivery names its event
| and carries the issue, and a comment's body in Atlassian's document
| format, read here as plain text.
|
*/

package trackers

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
jiraDelivery is a Jira webhook payload.
*/
type jiraDelivery struct {
	Event   string     `json:"webhookEvent"`
	Issue   *jiraIssue `json:"issue"`
	Comment *struct {
		Body   any `json:"body"`
		Author struct {
			DisplayName string `json:"displayName"`
		} `json:"author"`
	} `json:"comment"`
}

/*
Parse reads a Jira delivery: an issue that changed, or a comment.
*/
func (*Jira) Parse(body []byte) ([]Event, error) {
	var d jiraDelivery
	if err := decode(body, &d); err != nil {
		return nil, err
	}

	if d.Issue == nil {
		return nil, nil
	}

	if d.Comment != nil && strings.HasPrefix(d.Event, "comment_created") {
		return []Event{{
			Kind: EventComment, ExternalID: d.Issue.ID, Body: strings.TrimSpace(adfText(d.Comment.Body)),
			Author: d.Comment.Author.DisplayName,
		}}, nil
	}

	if strings.HasPrefix(d.Event, "jira:issue_") {
		is := d.Issue.issue("")

		return []Event{{Kind: EventIssue, ExternalID: d.Issue.ID, Issue: &is}}, nil
	}

	return nil, nil
}

/*
Subscribe registers the app's webhook for issues and comments on the
site, returning the id Jira gave it.
*/
func (j *Jira) Subscribe(ctx context.Context, c Call, address string) (string, error) {
	var out struct {
		Results []struct {
			ID int64 `json:"createdWebhookId"`
		} `json:"webhookRegistrationResult"`
	}

	err := j.call(ctx, c, request{method: fhttp.MethodPost, url: "/webhook", out: &out, body: map[string]any{
		"url": address,
		"webhooks": []any{map[string]any{
			"events":    []string{"jira:issue_updated", "jira:issue_deleted", "comment_created"},
			"jqlFilter": "project IS NOT EMPTY",
		}},
	}})
	if err != nil || len(out.Results) == 0 {
		return "", err
	}

	return formatInt(out.Results[0].ID), nil
}

/*
Refresh keeps the site's webhooks from lapsing.
*/
func (j *Jira) Refresh(ctx context.Context, c Call, ids []int64) error {
	return j.call(ctx, c, request{
		method: fhttp.MethodPut, url: "/webhook/refresh", body: map[string]any{"webhookIds": ids},
	})
}

/*
adf wraps plain text in Atlassian's document format, a paragraph per
line.
*/
func adf(text string) map[string]any {
	var paragraphs []any

	for line := range strings.SplitSeq(text, "\n") {
		p := map[string]any{"type": "paragraph", "content": []any{}}
		if line != "" {
			p["content"] = []any{map[string]any{"type": "text", "text": line}}
		}

		paragraphs = append(paragraphs, p)
	}

	return map[string]any{"type": "doc", "version": 1, "content": paragraphs}
}

/*
adfText reads the plain text of a document, or a body that is already
text.
*/
func adfText(node any) string {
	switch v := node.(type) {
	case string:
		return v
	case map[string]any:
		text, _ := v["text"].(string)
		children, _ := v["content"].([]any)

		parts := []string{text}
		for _, child := range children {
			parts = append(parts, adfText(child))
		}

		joined := strings.Join(parts, "")
		if v["type"] == "paragraph" {
			joined += "\n"
		}

		return joined
	}

	return ""
}
