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
| standing rules, the parts added when they apply, what a routine's run
| and its subagents are told, and the questions Operator is asked when
| it briefs and triages.
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
const promptVersion = "operator-20"

/*
The task files Operator reads, under resources/prompts/tasks/: the
standing rules every run opens with, who a teammate's run answers, what
a routine's run and its subagents are told, the parts of the system
prompt added for a thread about a conversation, for the workspace's own
instructions and for the teammate's memories, the reminder that closes
a question, how to draft a reply, what a run out of steps is told, and
the questions asked for a brief and a triage.
*/
const (
	taskSystem             = "operator/system"
	taskSession            = "operator/session"
	taskRoutine            = "operator/routine"
	taskSubagent           = "operator/subagent"
	taskRoutineSuggesting  = "operator/routine_suggesting"
	taskRoutineWindow      = "operator/routine_window"
	taskRoutineFirstRun    = "operator/routine_first_run"
	taskRoutineLastRun     = "operator/routine_last_run"
	taskSkills             = "operator/skills"
	taskThreadConversation = "operator/thread_conversation"
	taskInstructions       = "operator/workspace_instructions"
	taskMemories           = "operator/memories"
	taskWriteReminder      = "operator/write_reminder"
	taskReplyDrafting      = "operator/reply_drafting"
	taskLastCall           = "operator/last_call"
	taskBrief              = "operator/brief"
	taskTriage             = "operator/triage"
)

/*
systemPrompt is the run's system prompt: the standing rules, then who
the run answers (a teammate, or nobody when it runs a routine), then
the parts that apply to this run.
*/
func (e *Engine) systemPrompt(s *session) string {
	parts := []string{prompts.Task(taskSystem, nil), e.sessionPrompt(s)}
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
sessionPrompt says who the run answers: the teammate asking, or, in a
routine's run, the rules of working unattended as its main agent or as
one of its subagents, with the run's window and the skills on offer.
*/
func (e *Engine) sessionPrompt(s *session) string {
	today := e.Clock.Now().UTC().Format("Monday 2 January 2006")
	if s.routine == nil {
		return prompts.Task(taskSession, prompts.Vars{
			"workspace": s.m.Workspace.Name, "teammate": cmp.Or(s.askerName, "a teammate"), "role": s.m.Member.Role,
			"today": today,
		})
	}
	vars := prompts.Vars{
		"workspace": s.m.Workspace.Name, "today": today, "name": s.routine.row.Name,
		"window": e.runWindow(s), "skills": skillList(),
	}
	if s.sub != nil {
		return prompts.Task(taskSubagent, vars)
	}
	vars["suggesting"] = prompts.Task(taskRoutineSuggesting, nil)
	return prompts.Task(taskRoutine, vars)
}

/*
writeReminder closes every question from a teammate. The writing rules
sit far up in the prompt, and a long page of context after them makes a
model forget them; said again last, they hold.
*/
func writeReminder() string {
	return prompts.Task(taskWriteReminder, nil)
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
