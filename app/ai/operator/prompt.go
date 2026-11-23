/*
|--------------------------------------------------------------------------
| Operator Prompt
|--------------------------------------------------------------------------
|
| What Operator knows before it reads the thread: who it works for, the
| rules it keeps, the workspace's own instructions, what the teammate
| asked it to remember and the conversation the thread is about. The
| rules that keep customers safe are also enforced by the tools; the
| prompt only makes the model use them well.
|
| Every part is a file under resources/prompts/tasks/operator/: the
| standing rules, the parts added when they apply, and the questions
| Operator is asked when it briefs, triages or reviews procedures.
|
*/

package operator

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
promptVersion names the prompt the runs record.
*/
const promptVersion = "operator-2"

/*
The task files Operator reads, under resources/prompts/tasks/: the
standing rules every run opens with, the parts of the system prompt
added for a thread about a conversation, for the workspace's own
instructions and for the teammate's memories, and the questions asked
for a brief, a triage and a procedure review.
*/
const (
	taskSystem             = "operator/system"
	taskThreadConversation = "operator/thread_conversation"
	taskInstructions       = "operator/workspace_instructions"
	taskMemories           = "operator/memories"
	taskBrief              = "operator/brief"
	taskTriage             = "operator/triage"
	taskProcedureReview    = "operator/procedure_review"
)

/*
systemPrompt is the run's system prompt: the standing rules with the
workspace, the teammate and the date, then the parts that apply to
this run.
*/
func (e *Engine) systemPrompt(s *session) string {
	parts := []string{prompts.Task(taskSystem, prompts.Vars{
		"workspace": s.m.Workspace.Name, "teammate": cmp.Or(s.askerName, "a teammate"), "role": s.m.Member.Role,
		"today": e.Clock.Now().UTC().Format("Monday 2 January 2006"),
	})}
	if s.conversation != nil {
		number := strconv.FormatInt(s.conversation.Number, 10)
		parts = append(parts, prompts.Task(taskThreadConversation, prompts.Vars{"number": number}))
	}
	if text := strings.TrimSpace(s.settings.Instructions); text != "" {
		parts = append(parts, prompts.Task(taskInstructions, prompts.Vars{"instructions": text}))
	}
	if len(s.memories) > 0 {
		lines := make([]string, 0, len(s.memories))
		for _, m := range s.memories {
			lines = append(lines, "- "+m.Body)
		}
		parts = append(parts, prompts.Task(taskMemories, prompts.Vars{"memories": strings.Join(lines, "\n")}))
	}
	return strings.Join(parts, "\n\n")
}

/*
BriefQuestion is the question Operator answers when it briefs a
teammate on a conversation. A run is known to be a brief by being
asked exactly this.
*/
func BriefQuestion() string {
	return prompts.Task(taskBrief, nil)
}

/*
TriageQuestion is the question Operator answers when a conversation
arrives and the workspace asked it to triage new conversations.
*/
func TriageQuestion() string {
	return prompts.Task(taskTriage, nil)
}

/*
ProcedureQAQuestion is what Operator is asked when it reviews the
procedures by itself.
*/
func ProcedureQAQuestion() string {
	return prompts.Task(taskProcedureReview, nil)
}
