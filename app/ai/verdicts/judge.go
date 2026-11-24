/*
|--------------------------------------------------------------------------
| Draft Judge
|--------------------------------------------------------------------------
|
| While a room's agent only drafts, the team learns whether it could be
| trusted to answer by itself: each draft is compared with the reply the
| teammate actually sent. The cheap model says whether the two give the
| customer the same answer, wording aside. The agreement rate over the
| latest drafts is what a teammate reads before letting the agent answer.
| The instructions are verdicts/draft_judge.md.
|
*/

package verdicts

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
maxJudgeRunes is the most of each text the judge reads.
*/
const maxJudgeRunes = 2000

/*
judgeSchema is the shape of a judge's verdict.
*/
const judgeSchema = `{"type": "object", "additionalProperties": false, "required": ["agree", "reason"],
"properties": {"agree": {"type": "boolean"}, "reason": {"type": "string"}}}`

/*
Judge compares drafts with the replies teammates sent.
*/
type Judge struct {
	Verdicts *Verdicts
}

/*
Agree reports whether the draft gives the same answer as the reply,
recording the question as a judge run on the conversation.
*/
func (j *Judge) Agree(ctx context.Context, in channels.DraftComparison) (bool, error) {
	var verdict struct {
		Agree  bool   `json:"agree"`
		Reason string `json:"reason"`
	}

	err := j.Verdicts.ask(ctx, question{
		workspaceID: in.WorkspaceID, kind: "judge", conversationID: &in.ConversationID,
		system: prompts.Task(taskDraftJudge, nil), schemaName: "draft_verdict", schema: json.RawMessage(judgeSchema),
		text: "DRAFT:\n" + cut(in.Draft, maxJudgeRunes) + "\n\nREPLY:\n" + cut(in.Reply, maxJudgeRunes),
	}, &verdict)

	return verdict.Agree, err
}
