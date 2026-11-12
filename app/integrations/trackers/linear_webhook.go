/*
|--------------------------------------------------------------------------
| Linear Deliveries
|--------------------------------------------------------------------------
|
| Linear delivers one action per request: an issue created, updated or
| removed, or a comment. An issue's delivery carries its state and team,
| so the link updates without asking Linear again; a comment carries
| its issue's id, its body and who wrote it.
|
*/

package trackers

import (
	"encoding/json"
	"fmt"
)

/*
linearDelivery is a Linear webhook payload.
*/
type linearDelivery struct {
	Action string          `json:"action"`
	Type   string          `json:"type"`
	URL    string          `json:"url"`
	Data   json.RawMessage `json:"data"`
}

/*
linearComment is a comment's data.
*/
type linearComment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	IssueID string `json:"issueId"`
	User    struct {
		Name string `json:"name"`
	} `json:"user"`
}

/*
Parse reads a Linear delivery: an issue that changed, or a comment.
*/
func (*Linear) Parse(body []byte) ([]Event, error) {
	var d linearDelivery
	if err := decode(body, &d); err != nil {
		return nil, err
	}

	switch d.Type {
	case "Issue":
		var is linearIssue
		if err := decode(d.Data, &is); err != nil {
			return nil, err
		}

		if is.URL == "" {
			is.URL = d.URL
		}

		issue := is.issue()

		return []Event{{Kind: EventIssue, ExternalID: is.ID, Issue: &issue}}, nil
	case "Comment":
		if d.Action != "create" {
			return nil, nil
		}

		var c linearComment
		if err := decode(d.Data, &c); err != nil {
			return nil, err
		}

		return []Event{{Kind: EventComment, ExternalID: c.IssueID, Body: c.Body, Author: c.User.Name}}, nil
	}

	return nil, nil
}

/*
decode reads JSON, naming the failure as the tool's.
*/
func decode(body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %w", ErrProvider, err)
	}

	return nil
}
