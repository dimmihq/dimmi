/*
|--------------------------------------------------------------------------
| Outcomes
|--------------------------------------------------------------------------
|
| How the conversation ended: its closes and reopens, who held it at the
| last close, its rating and what the AI did. A rating goes to the
| teammate it rated, or to dimmi when no teammate ever replied. An AI
| resolution is confirmed when the customer said an answer helped or
| rated four or five with no teammate reply.
|
*/

package reports

/*
goodRating is the lowest score that counts as satisfied.
*/
const goodRating = 4

/*
applyCloses fills the resolution facts.
*/
func applyCloses(row *FactRow, h history, d durations) {
	row.Closes = len(h.closes)
	row.Reopens = h.reopens
	if len(h.closes) == 0 {
		return
	}

	row.FirstClosedAt = new(h.closes[0])
	row.FirstCloseSeconds, row.FirstCloseBusinessSeconds = d.span(row.StartedAt, row.FirstClosedAt)

	if row.Status != statusClosed {
		return
	}

	last := h.closes[len(h.closes)-1]
	row.ClosedAt = &last
	row.ResolutionSeconds, row.ResolutionBusinessSeconds = d.span(row.StartedAt, row.ClosedAt)
	row.FirstContactResolved = row.Reopens == 0 && row.HumanReplies <= 1

	holder := h.at(last)
	row.ClosedByMemberID, row.ClosedByTeamID = holder.member, holder.team
}

/*
applyRating fills the latest rating.
*/
func applyRating(row *FactRow, in Input) {
	if in.Rating == nil {
		return
	}

	row.Rating = new(in.Rating.Score)
	row.RatedAt = new(in.Rating.RatedAt)
	if row.HumanReplies > 0 {
		row.RatedMemberID = in.Rating.MemberID
	}
}

/*
applyAI fills what the AI agent did.
*/
func applyAI(row *FactRow, in Input, s messageSummary, h history) {
	row.AiAnswers = s.aiAnswers
	row.AiInvolved = s.aiAnswers > 0
	row.AiFirstAnswerAt, row.AiLastAnswerAt = s.aiFirst, s.aiLast
	row.AiHandedOffAt, row.AiHandoffReason = h.handoffAt, h.handoffReason

	if !row.AiInvolved || row.HumanReplies > 0 || row.AiHandedOffAt != nil {
		return
	}

	row.AiConfirmed = in.Helped || (row.Rating != nil && *row.Rating >= goodRating)
}
