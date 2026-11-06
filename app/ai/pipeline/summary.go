/*
|--------------------------------------------------------------------------
| Handoff Summary
|--------------------------------------------------------------------------
|
| A teammate taking over reads one note first: what the customer wants,
| what the agent already tried, their mood, and why it was handed over.
| The cheap model writes it after the handoff is committed; when it
| cannot, a plain note built from the conversation stands in, so every
| handoff carries a note and raises its event.
|
*/

package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
afterHandoff writes the summary note, adds its cost to the run and
the month, and raises the handoff event. A failure here only costs
the note.
*/
func (r *Runner) afterHandoff(ctx context.Context, st *run, d *decision) {
	rec := st.rec
	before := spendDelta{
		cost: rec.costMicros, input: rec.tokens.input, cached: rec.tokens.cached, output: rec.tokens.output,
	}

	summary := r.summarize(ctx, st, d)
	delta := spendDelta{
		cost: rec.costMicros - before.cost, input: rec.tokens.input - before.input,
		cached: rec.tokens.cached - before.cached, output: rec.tokens.output - before.output,
	}

	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e := st.env
		if _, err := r.agentWriter(tx, e).AddNote(ctx, e.Conversation, summary); err != nil {
			return err
		}
		if err := r.finishRun(ctx, tx, st, d); err != nil {
			return err
		}
		if err := r.addSpend(ctx, tx, st, delta); err != nil {
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
			Reason:          d.handoffReason, Summary: summary,
		})
	})
}

/*
summarize asks the answer provider's cheap model for the note, falling back to the
plain one.
*/
func (r *Runner) summarize(ctx context.Context, st *run, d *decision) string {
	e := st.env
	fallback := plainSummary(st, d)

	llm, model, key := r.cheapLLM(ctx, e)
	if llm == nil {
		return fallback
	}

	system := r.render(e, "summary.system.md", prompts.Vars{
		"language": languageName(e.Agent.DefaultLanguage), "reason": d.handoffReason,
	})
	user := r.summaryConversation(st, d) + "\n" + sourcesUsed(st, d)
	res, err := r.chat(ctx, st, jsonCall{
		kind: enums.AIRunStepKindSummarize, llm: llm, key: key, model: model, system: system,
		user: user, timeout: r.Timeouts.SummarizeTotal, maxTokens: 500,
	}, []contracts.ChatMessage{userMessage(user)})
	if err != nil || strings.TrimSpace(res.Text) == "" {
		return fallback
	}
	return strings.TrimSpace(res.Text)
}

/*
summaryConversation is the whole exchange the note covers: history,
the pending messages and the reply just posted.
*/
func (r *Runner) summaryConversation(st *run, d *decision) string {
	e := st.env
	var b strings.Builder
	b.WriteString("<conversation>\n")
	for _, m := range append(append([]models.Message{}, e.History...), e.Pending...) {
		text := truncate(r.redactFor(e, body(m)), turnChars)
		fmt.Fprintf(&b, "<turn role=\"%s\">%s</turn>\n", roleOf(m), escape(text))
	}
	if text := r.reply(e, d); text != "" {
		fmt.Fprintf(&b, "<turn role=\"assistant\">%s</turn>\n", escape(text))
	}
	b.WriteString("</conversation>")
	return b.String()
}

/*
sourcesUsed lists the titles of the sources the run cited, or kept
when it cited none.
*/
func sourcesUsed(st *run, d *decision) string {
	var titles []string
	for _, c := range d.citations {
		titles = append(titles, c.Title)
	}
	if len(titles) == 0 {
		for _, s := range st.sources {
			titles = append(titles, s.chunk.Title)
		}
	}
	return "<sources_used>" + escape(strings.Join(titles, "\n")) + "</sources_used>"
}

/*
plainSummary is the note written without a model.
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
