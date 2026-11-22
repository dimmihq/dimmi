/*
|--------------------------------------------------------------------------
| What A Procedure Names
|--------------------------------------------------------------------------
|
| A procedure names things outside itself: the actions its chips call,
| the articles it cites, the teams it hands over to, tags, ticket types,
| saved audiences and the sub-procedures it follows. Publishing checks
| each one still exists, and the runtime offers every action. Where a
| procedure applies and how its handovers are routed live here too.
|
*/

package procedure

import "slices"

/*
HandoverConfig routes the handovers a procedure causes.
*/
type HandoverConfig struct {
	TeamID   string `json:"team_id,omitempty"`
	Priority string `json:"priority,omitempty"`
}

/*
Scope narrows where a procedure applies: saved audiences, channels,
inboxes, signed-in customers only, and whether it is only ever followed
from inside another procedure.
*/
type Scope struct {
	AudienceIDs             []string `json:"audience_ids,omitempty"`
	Channels                []string `json:"channels,omitempty"`
	InboxIDs                []string `json:"inbox_ids,omitempty"`
	RequireVerifiedIdentity bool     `json:"require_verified_identity,omitempty"`
	ReusableOnly            bool     `json:"reusable_only,omitempty"`
}

/*
ActionIDs lists the actions the definition's steps take, nested steps
and chips included, once each, in reading order.
*/
func ActionIDs(d Definition) []string {
	return References(d).Actions
}

/*
Refs are the things a definition names, by kind, once each.
*/
type Refs struct {
	Actions       []string
	Attributes    []string
	Articles      []string
	Teams         []string
	Tags          []string
	TicketTypes   []string
	SubProcedures []string
}

/*
References lists what the definition names, for publishing to check.
*/
func References(d Definition) Refs {
	var out Refs
	add := func(list *[]string, ref string) {
		if ref != "" && !slices.Contains(*list, ref) {
			*list = append(*list, ref)
		}
	}

	Walk(Normalize(d).Steps, func(s Step, _ int) {
		if s.Kind == KindSubProcedure {
			add(&out.SubProcedures, s.SubProcedure)
		}
		for _, b := range s.Bindings {
			switch b.Kind {
			case BindAction:
				add(&out.Actions, b.Ref)
			case BindAttribute:
				add(&out.Attributes, b.Ref)
			case BindArticle:
				add(&out.Articles, b.Ref)
			case BindHandoff:
				add(&out.Teams, b.Ref)
			case BindTag:
				add(&out.Tags, b.Ref)
			case BindTicketType:
				add(&out.TicketTypes, b.Ref)
			case BindSubProcedure:
				add(&out.SubProcedures, b.Ref)
			}
		}
	})

	if d.HandoverRule != nil {
		add(&out.Teams, d.HandoverRule.TeamID)
	}

	return out
}

/*
StepWith returns the first step that names a reference of a kind, by
its id.
*/
func StepWith(d Definition, kind, ref string) string {
	found := ""
	Walk(Normalize(d).Steps, func(s Step, _ int) {
		if found != "" {
			return
		}
		if kind == BindSubProcedure && s.SubProcedure == ref {
			found = s.ID
		}
		for _, b := range s.Bindings {
			if b.Kind == kind && b.Ref == ref {
				found = s.ID
			}
		}
	})

	return found
}
