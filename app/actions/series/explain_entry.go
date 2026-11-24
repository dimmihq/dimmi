/*
|--------------------------------------------------------------------------
| Judging A Person's Entry Now
|--------------------------------------------------------------------------
|
| Why someone never entered a series is read from how its live version
| judges them right now: the entry rules or saved audience, condition by
| condition with the value the person has, the entry event, the exit
| rules that keep a match out, and what keeps anyone out regardless.
| Rules are tested as the teammate, the way "Would Ada match?" tests an
| audience.
|
*/

package series

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
EntryCheck is how the live series' rules judge the contact now: the
saved audience the entry names, the entry rules (or the audience's), the
entry event and whether they did it since the series went live, the
exit rules, and what keeps anyone out regardless: being blocked, or
having no company on an account series.
*/
type EntryCheck struct {
	Audience  *models.Audience
	Rules     *RuleCheck
	Event     string
	EventDone bool
	Exit      *RuleCheck
	Blocked   bool
	NoCompany bool
}

/*
RuleCheck is one set of rules tested on the contact: "true", "false",
or "unknown", with every condition's result. Problem says why it could
not be tested.
*/
type RuleCheck struct {
	Result  string
	Leaves  []audience.LeafResult
	Problem string
}

/*
entryCheck tests the live series' entry and exit on the contact now.
*/
func (s *Series) entryCheck(ctx context.Context, m authz.Membership, story Story, c models.Contact) *EntryCheck {
	spec := *story.LiveSpec
	out := &EntryCheck{
		Event: spec.Entry.Event, Blocked: c.BlockedAt != nil,
		NoCompany: story.Series.Subject == SubjectCompany && c.CompanyID == nil,
	}

	rules := spec.Entry.Rules
	if a, ok := s.entryAudience(ctx, m.Workspace.ID, spec.Entry.AudienceID); ok {
		out.Audience, rules = &a, a.Rules
	}

	if hasRules(rules) {
		out.Rules = s.ruleCheck(ctx, m, c, rules)
	}

	if hasRules(spec.Exit.Rules) {
		out.Exit = s.ruleCheck(ctx, m, c, spec.Exit.Rules)
	}

	if spec.Entry.Event != "" {
		out.EventDone = s.didSinceLive(ctx, story.Series, c, spec.Entry.Event)
	}

	return out
}

/*
entryAudience reads the saved audience an entry names.
*/
func (s *Series) entryAudience(ctx context.Context, ws ids.UUID, public string) (models.Audience, bool) {
	id := s.audienceID(public)
	if id == nil {
		return models.Audience{}, false
	}

	var a models.Audience

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		a, err = q.FindAudience(ctx, models.FindAudienceParams{WorkspaceID: ws, ID: *id})

		return err
	})

	return a, err == nil
}

/*
didSinceLive reports whether the contact did the event since the series
went live.
*/
func (s *Series) didSinceLive(ctx context.Context, row models.Series, c models.Contact, event string) bool {
	since := row.CreatedAt
	if row.PublishedAt != nil {
		since = *row.PublishedAt
	}

	var did bool

	_ = s.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		var err error
		did, err = q.ContactEventSince(ctx, models.ContactEventSinceParams{
			WorkspaceID: row.WorkspaceID, ContactIds: []ids.UUID{c.ID}, Name: event, Since: since,
		})

		return err
	})

	return did
}

/*
ruleCheck tests one set of rules on the contact with every condition.
*/
func (s *Series) ruleCheck(ctx context.Context, m authz.Membership, c models.Contact, rules json.RawMessage,
) *RuleCheck {
	if s.Audiences == nil {
		return &RuleCheck{Problem: "rules cannot be tested here"}
	}

	got, err := s.Audiences.Evaluate(ctx, audiences.EvaluateInput{Membership: m, ContactID: c.ID, Rules: rules})
	switch {
	case errors.Is(err, exceptions.ErrForbidden):
		return &RuleCheck{Problem: "the teammate's role may not test audiences on contacts"}
	case err != nil:
		return &RuleCheck{Problem: err.Error()}
	}

	return &RuleCheck{Result: got.Result, Leaves: got.Leaves}
}

/*
hasRules reports whether rules say anything.
*/
func hasRules(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text != "" && text != "null" && text != "{}"
}
