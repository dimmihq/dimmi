/*
|--------------------------------------------------------------------------
| Pick Assignee
|--------------------------------------------------------------------------
|
| The one place that decides who gets a conversation nobody holds.
| Round robin takes turns in member order; balanced gives it to whoever
| holds the fewest open conversations, the one who waited longest for
| their last breaking a tie. Either way only active members of the pool
| who are online, unless the inbox lets away members take conversations,
| and under every limit they have are eligible.
|
| The pool's cursor row is locked before anyone is chosen, so two
| conversations arriving together are handed out one after the other and
| never to the same free slot. When the pool has nobody, the overflow
| team is tried once; when that has nobody either, the conversation waits
| for the reason the first pool gave.
|
*/

package assignment

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The outcomes of a pick and why a member may not take a conversation.
*/
const (
	OutcomeAssigned          = "assigned"
	OutcomeNoAvailableMember = "no_available_member"
	OutcomeAllAtCapacity     = "all_at_capacity"
	WhyAway                  = "away"
	WhyAtCapacity            = "at_capacity"
	WhyAtChannelCapacity     = "at_channel_capacity"
)

/*
PickInput is what a pick needs: the workspace and inbox, the team whose
members form the pool, nil for the whole workspace, the inbox's
assignment settings and the channel type the conversation came through.
*/
type PickInput struct {
	WorkspaceID ids.UUID
	InboxID     ids.UUID
	TeamID      *ids.UUID
	Settings    models.AssignmentSettings
	ChannelType enums.ChannelType
	Now         time.Time
}

/*
PickResult is the member picked, nil when nobody was, and the outcome.
*/
type PickResult struct {
	MemberID *ids.UUID
	Outcome  string
}

/*
Candidate is one member of a pool and whether they may take a
conversation now: WhyNot is empty when they may.
*/
type Candidate struct {
	models.ListPoolCandidatesRow
	Capacity        *int
	ChannelCapacity *int
	WhyNot          string
}

/*
PickAssignee picks a member for a conversation inside the caller's
transaction, trying the overflow team when the pool has nobody, and
records the pick on the pool's cursor and the member.
*/
func PickAssignee(ctx context.Context, q *models.Queries, in PickInput) (PickResult, error) {
	first, err := pickIn(ctx, q, in)
	if err != nil || first.MemberID != nil || in.Settings.OverflowTeamID == nil {
		return first, err
	}

	overflow := in
	overflow.TeamID = in.Settings.OverflowTeamID

	second, err := pickIn(ctx, q, overflow)
	if err != nil || second.MemberID != nil {
		return second, err
	}

	return first, nil
}

/*
pickIn picks a member of one pool under its locked cursor.
*/
func pickIn(ctx context.Context, q *models.Queries, in PickInput) (PickResult, error) {
	kind, poolID := pool(in)

	err := q.EnsureAssignmentCursor(ctx, models.EnsureAssignmentCursorParams{
		PoolKind: kind, PoolID: poolID, WorkspaceID: in.WorkspaceID, Now: in.Now,
	})
	if err != nil {
		return PickResult{}, fmt.Errorf("create assignment cursor: %w", err)
	}

	cursor, err := q.LockAssignmentCursor(ctx, models.LockAssignmentCursorParams{
		WorkspaceID: in.WorkspaceID, PoolKind: kind, PoolID: poolID,
	})
	if err != nil {
		return PickResult{}, fmt.Errorf("lock assignment cursor: %w", err)
	}

	candidates, err := Candidates(ctx, q, in)
	if err != nil {
		return PickResult{}, err
	}

	chosen, outcome := choose(in.Settings.Mode, candidates, cursor.LastMemberID)
	if chosen == nil {
		return PickResult{Outcome: outcome}, nil
	}

	return PickResult{MemberID: chosen, Outcome: OutcomeAssigned}, record(ctx, q, in, *chosen)
}

/*
pool names the pool a pick draws from: the team, or the inbox when no
team holds the conversation.
*/
func pool(in PickInput) (string, ids.UUID) {
	if in.TeamID != nil {
		return "team", *in.TeamID
	}

	return "inbox", in.InboxID
}

/*
record moves the pool's cursor to the member and notes when they were
last given a conversation.
*/
func record(ctx context.Context, q *models.Queries, in PickInput, member ids.UUID) error {
	kind, poolID := pool(in)

	err := q.AdvanceAssignmentCursor(ctx, models.AdvanceAssignmentCursorParams{
		MemberID: &member, Now: &in.Now, WorkspaceID: in.WorkspaceID, PoolKind: kind, PoolID: poolID,
	})
	if err != nil {
		return fmt.Errorf("advance assignment cursor: %w", err)
	}

	err = q.SetMemberLastAssigned(ctx, models.SetMemberLastAssignedParams{
		Now: &in.Now, WorkspaceID: in.WorkspaceID, ID: member,
	})
	if err != nil {
		return fmt.Errorf("record last assignment: %w", err)
	}

	return nil
}

/*
Candidates returns the members of the pool with whether each may take
a conversation now and why not.
*/
func Candidates(ctx context.Context, q *models.Queries, in PickInput) ([]Candidate, error) {
	rows, err := q.ListPoolCandidates(ctx, models.ListPoolCandidatesParams{
		ChannelType: string(in.ChannelType), WorkspaceID: in.WorkspaceID, TeamID: in.TeamID,
	})
	if err != nil {
		return nil, fmt.Errorf("list pool candidates: %w", err)
	}

	out := make([]Candidate, len(rows))
	for i, row := range rows {
		out[i] = candidate(row, in)
	}

	return out, nil
}

/*
candidate works out the member's limits and whether they may take a
conversation: their own limit wins over the inbox's.
*/
func candidate(row models.ListPoolCandidatesRow, in PickInput) Candidate {
	c := Candidate{ListPoolCandidatesRow: row, Capacity: in.Settings.DefaultCapacity}

	if row.ConversationCapacity != nil {
		limit := int(*row.ConversationCapacity)
		c.Capacity = &limit
	}

	own := map[string]int{}
	_ = json.Unmarshal(row.CapacityByChannel, &own)

	if limit, ok := own[string(in.ChannelType)]; ok {
		c.ChannelCapacity = &limit
	} else if limit, ok := in.Settings.CapacityByChannel[string(in.ChannelType)]; ok {
		c.ChannelCapacity = &limit
	}

	switch {
	case row.Availability != string(enums.AvailabilityOnline) && !in.Settings.IncludeAway:
		c.WhyNot = WhyAway
	case c.Capacity != nil && int(row.OpenCount) >= *c.Capacity:
		c.WhyNot = WhyAtCapacity
	case c.ChannelCapacity != nil && int(row.ChannelOpenCount) >= *c.ChannelCapacity:
		c.WhyNot = WhyAtChannelCapacity
	}

	return c
}

/*
choose picks among the candidates by the mode, and says why nobody was
picked when nobody was.
*/
func choose(mode string, candidates []Candidate, last *ids.UUID) (*ids.UUID, string) {
	var eligible []Candidate

	available := false

	for _, c := range candidates {
		if c.WhyNot != WhyAway {
			available = true
		}

		if c.WhyNot == "" {
			eligible = append(eligible, c)
		}
	}

	if len(eligible) == 0 {
		if available {
			return nil, OutcomeAllAtCapacity
		}

		return nil, OutcomeNoAvailableMember
	}

	if mode == models.AssignmentBalanced {
		return balanced(eligible), OutcomeAssigned
	}

	return roundRobin(eligible, last), OutcomeAssigned
}

/*
roundRobin takes the first eligible member after the last one picked,
in member order, starting over at the end.
*/
func roundRobin(eligible []Candidate, last *ids.UUID) *ids.UUID {
	if last != nil {
		for _, c := range eligible {
			if c.ID.String() > last.String() {
				return &c.ID
			}
		}
	}

	return &eligible[0].ID
}

/*
balanced takes the member with the fewest open conversations, then the
one given a conversation longest ago, then the lowest member id.
*/
func balanced(eligible []Candidate) *ids.UUID {
	best := slices.MinFunc(eligible, func(a, b Candidate) int {
		return cmp.Or(
			cmp.Compare(a.OpenCount, b.OpenCount),
			compareTimes(a.LastAssignedAt, b.LastAssignedAt),
			cmp.Compare(a.ID.String(), b.ID.String()),
		)
	})

	return &best.ID
}

/*
compareTimes orders never before any time, and earlier before later.
*/
func compareTimes(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}

	return a.Compare(*b)
}
