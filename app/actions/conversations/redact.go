/*
|--------------------------------------------------------------------------
| Redacting What Is Written
|--------------------------------------------------------------------------
|
| A message's text, its HTML and a conversation's subject pass through
| the workspace's redaction rules before they are stored, whoever writes
| them: a customer on any channel, a teammate's reply or note, or the
| system. The message keeps only which rules hit and how often, and the
| channel that brought it in hears that it should drop its raw copy.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/redaction"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	supportredaction "github.com/dimmihq/dimmi/app/support/redaction"
)

/*
redactText applies the workspace's rules to the texts in place and
returns the hits.
*/
func redactText(
	ctx context.Context, q *models.Queries, workspaceID ids.UUID, texts ...*string,
) ([]supportredaction.Hit, error) {
	r, err := redaction.Load(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}

	return r.ApplyAll(texts...), nil
}

/*
redactMessage applies the workspace's rules to the entry's text and HTML
and notes the hits in its metadata, which defaults to an empty object.
*/
func (w *Writer) redactMessage(
	ctx context.Context, workspaceID ids.UUID, m *NewMessage,
) ([]supportredaction.Hit, error) {
	if m.Metadata == nil {
		m.Metadata = json.RawMessage(`{}`)
	}

	hits, err := redactText(ctx, w.Q, workspaceID, &m.Body, m.BodyHTML)
	if err != nil || len(hits) == 0 {
		return nil, err
	}

	metadata := map[string]any{}
	if err := json.Unmarshal(m.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("read message metadata: %w", err)
	}

	metadata["redactions"] = hits

	if m.Metadata, err = json.Marshal(metadata); err != nil {
		return nil, fmt.Errorf("write message metadata: %w", err)
	}

	return hits, nil
}

/*
RedactEdit applies the workspace's rules to a rewritten text in place
and returns the hits, so an edit is held to the same rules as the text
it replaces.
*/
func (w *Writer) RedactEdit(ctx context.Context, workspaceID ids.UUID, text *string) ([]supportredaction.Hit, error) {
	return redactText(ctx, w.Q, workspaceID, text)
}

/*
AnnounceRedaction tells the application which rules changed the
message, and stays quiet when none did.
*/
func (w *Writer) AnnounceRedaction(
	ctx context.Context, c models.Conversation, msg models.Message, hits []supportredaction.Hit,
) error {
	if len(hits) == 0 {
		return nil
	}

	rules := make([]string, 0, len(hits))
	for _, hit := range hits {
		rules = append(rules, hit.Rule)
	}

	return w.Dispatch(ctx, appevents.MessageRedacted{
		Base:       w.Base(c.WorkspaceID),
		MessageRef: appevents.MessageRef{ConversationRef: ref(c), MessageID: msg.ID},
		Rules:      rules, Count: supportredaction.Count(hits),
	})
}

/*
copyText returns a copy of the text, so redacting it leaves the caller's
input as it was.
*/
func copyText(text *string) *string {
	if text == nil {
		return nil
	}

	out := *text

	return &out
}
