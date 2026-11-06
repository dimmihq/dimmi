/*
|--------------------------------------------------------------------------
| Handoff Note
|--------------------------------------------------------------------------
|
| A teammate taking over reads one note first: what the customer wants,
| what the agent tried, their mood, and why it was handed over. The
| agent writes it as part of its handoff call; when it did not — a
| policy handoff, a failed run, an empty note — a plain note built from
| the conversation stands in, so every handoff carries a note and
| raises its event.
|
*/

package pipeline

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
writeNote adds the handoff note and raises the handoff event, inside
the commit's transaction.
*/
func (r *Runner) writeNote(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	note := plainSummary(st, d)
	if st.handoff != nil && strings.TrimSpace(st.handoff.note) != "" {
		note = strings.TrimSpace(markerPattern().ReplaceAllString(st.handoff.note, ""))
	}
	if _, err := r.agentWriter(tx, e).AddNote(ctx, e.Conversation, note); err != nil {
		return err
	}
	if r.Events == nil {
		return nil
	}
	return r.Events.Dispatch(ctx, tx, appevents.AiHandoffRequested{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: e.Workspace.ID, OccurredAt: r.Clock.Now(),
			Actor: appevents.Actor{Type: string(enums.AuthorTypeAgent), ID: e.Agent.ID},
		},
		ConversationRef: appevents.ConversationRef{ConversationID: e.Conversation.ID},
		Reason:          d.handoffReason, Summary: note,
	})
}

/*
plainSummary is the note written without the agent.
*/
func plainSummary(st *run, d *decision) string {
	wants := truncate(strings.ReplaceAll(st.env.pendingText(), "\n", " "), 300)
	tried := "Nothing yet."
	var titles []string
	for _, s := range st.sources {
		titles = append(titles, s.chunk.Title)
	}
	if len(titles) > 0 {
		tried = "Searched the knowledge base: " + strings.Join(titles, ", ") + "."
	}
	return strings.Join([]string{
		"Wants: " + wants,
		"Tried: " + tried,
		"Mood: not assessed.",
		"Handed off because: " + strings.ReplaceAll(d.handoffReason, "_", " ") + ".",
	}, "\n")
}
