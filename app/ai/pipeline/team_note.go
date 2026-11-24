/*
|--------------------------------------------------------------------------
| Note for the Team
|--------------------------------------------------------------------------
|
| Not everything the team should know needs a person to step in. A
| buyer who has decided against buying, a feature someone asked for
| that is not offered, a customer who mentioned they are moving
| offices next month: the team wants to read these, and nobody has to
| take the conversation over to do it.
|
| Until this tool the agent's only way to tell the team anything was a
| handoff, which gives the conversation away, stops the agent replying
| and pulls a person in. So "let the team know" always became "hand it
| to the team", and a rep was assigned a conversation with a buyer who
| had already said no.
|
| leave_note writes an internal note on the conversation, where the
| team reads it in the inbox beside the thread. The conversation stays
| with the agent, which keeps replying. The customer never sees the
| note. The tool records it; the commit writes it with the agent as
| author, so the thread shows who left it.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
toolNote is the note tool's name.
*/
const toolNote = "leave_note"

/*
Note limits: the notes one turn may leave, and a note's length in
runes.
*/
const (
	notesPerTurn = 2
	noteRunes    = 1200
)

/*
noteKept tells the agent what a note is not. A result reading only
"ok" looks like the team was told and will act, and an agent that
believes so tells the customer someone will be in touch. It states
what happened and nothing about what to tell the customer: that is the
team's to decide in its guidance, and a line here about the reply is
read after the guidance and wins over it.
*/
const noteKept = "The note is on the conversation for the team to read. Nobody was asked to reply and nobody " +
	"is taking over: the conversation stays with you."

/*
noteSpec describes leave_note.
*/
func noteSpec() contracts.ToolSpec {
	return contracts.ToolSpec{
		Name: toolNote,
		Description: "Leave an internal note on this conversation for the team, without handing it over. The team " +
			"reads it in the inbox; the customer never sees it; you keep the conversation and keep replying. Use " +
			"it when the team should know something and no person needs to step in now, or when the workspace's " +
			"guidance says to note something. It is not a handoff: when a person must take over, use handoff.",
		Schema: objectSchema(map[string]any{
			"note": textProperty("What the team should know, in the workspace's language: what the customer said " +
				"or wants, with every detail they gave, and why it is worth the team's attention."),
		}, "note"),
	}
}

/*
leaveNote records a note for the team, written when the turn commits.
*/
func (r *Runner) leaveNote(_ context.Context, st *run, args json.RawMessage) (string, error) {
	var in struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", errToolArguments
	}
	note := strings.TrimSpace(markerPattern().ReplaceAllString(in.Note, ""))
	if note == "" {
		return "", errToolArguments
	}
	if len(st.teamNotes) >= notesPerTurn {
		return `{"ok":false,"reason":"this reply has already left its notes; add nothing more"}`, nil
	}
	note = truncate(note, noteRunes)
	st.teamNotes = append(st.teamNotes, note)
	if st.sim != nil {
		st.sim.calls = append(st.sim.calls, procedure.ObservedCall{
			Tool: toolNote, Args: map[string]any{"note": note}, Succeeded: true,
		})
	}
	out, err := json.Marshal(map[string]any{"ok": true, "note": noteKept})
	return string(out), err
}

/*
writeTeamNotes adds the turn's notes to the conversation as the agent,
inside the commit's transaction.
*/
func (r *Runner) writeTeamNotes(ctx context.Context, tx *database.Tx, st *run) error {
	w := r.agentWriter(tx, st.env)
	for _, note := range st.teamNotes {
		if _, err := w.AddNote(ctx, st.env.Conversation, note); err != nil {
			return fmt.Errorf("leave note: %w", err)
		}
	}
	return nil
}
