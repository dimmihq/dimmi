/*
|--------------------------------------------------------------------------
| Testing A Person
|--------------------------------------------------------------------------
|
| "Would Ada match?" is answered with every condition: the value Ada
| actually has, how far it is trusted, and whether it held. A page,
| language, device and country can be typed in to stand for the
| browser; page conditions with nothing typed in stay open, so the
| answer says it depends on the page instead of guessing. Nothing is
| written: testing never acts as the person.
|
*/

package audiences

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
ErrContactNotFound refuses a test of a contact the workspace lacks.
*/
var ErrContactNotFound = fault.New(fault.NotFound, "contact_not_found", "No contact of this workspace has this id.").
	OnField("contact_id")

/*
Browser is what a browser would report, typed in to test a person.
*/
type Browser struct {
	PageURL string
	Locale  string
	Device  string
	Country string
}

/*
EvaluateInput is a rule and a person to test it on.
*/
type EvaluateInput struct {
	Membership authz.Membership
	ContactID  ids.UUID
	Rules      json.RawMessage
	Browser    Browser
}

/*
Evaluation is whether the person matches: "true", "false", or
"unknown" when it depends on the page. Leaves explain every condition.
*/
type Evaluation struct {
	Result  string
	Leaves  []audience.LeafResult
	Contact models.Contact
}

/*
Evaluate tests a rule on one person.
*/
func (a *Audiences) Evaluate(ctx context.Context, in EvaluateInput) (Evaluation, error) {
	m := in.Membership
	if !m.Can("audiences:read") || !m.Can("contacts:read") {
		return Evaluation{}, exceptions.ErrForbidden
	}

	parsed, err := Parse(in.Rules)
	if err != nil {
		return Evaluation{}, err
	}

	program, _ := audience.Compile(parsed, audience.Base(), audience.SavedAudience())
	var out Evaluation

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		person, err := loadPerson(ctx, q, m.Workspace.ID, in.ContactID)
		if err != nil {
			return err
		}

		env, err := a.env(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}

		person.Client = browserValues(in.Browser)
		env.Facts = person
		env.Unknown = func(f audience.Field) bool {
			_, typed := person.Client[f.Key]
			return f.Where == audience.Client && !typed
		}

		explained := program.Explain(env)
		out = Evaluation{
			Result: explained.Result.String(), Leaves: trusted(explained.Leaves, person.Contact),
			Contact: person.Contact,
		}

		return nil
	})

	return out, err
}

/*
loadPerson reads one contact as rules see them.
*/
func loadPerson(ctx context.Context, q *models.Queries, ws, id ids.UUID) (audiences.Person, error) {
	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) || (err == nil && contact.DeletedAt != nil) {
		return audiences.Person{}, ErrContactNotFound
	}

	if err != nil {
		return audiences.Person{}, fmt.Errorf("find contact: %w", err)
	}

	people, err := audiences.LoadPeople(ctx, q, ws, []models.Contact{contact})
	if err != nil {
		return audiences.Person{}, fmt.Errorf("load person: %w", err)
	}

	return people[0], nil
}

/*
browserValues turns what was typed in into page, visitor and session
values, leaving out what was not.
*/
func browserValues(b Browser) map[string]any {
	out := map[string]any{}
	put := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			out[key] = value
		}
	}

	put("visitor.locale", b.Locale)
	put("visitor.device", strings.ToLower(b.Device))
	put("session.country", strings.ToUpper(b.Country))

	if u, err := url.Parse(strings.TrimSpace(b.PageURL)); err == nil && u.Host != "" {
		u.Fragment = ""
		put("page.url", u.String())
		put("page.path", u.EscapedPath())
		put("page.query", u.RawQuery)
		put("page.host", u.Host)
	}

	return out
}

/*
trusted marks a custom attribute verified when the person's value came
from the team's own systems rather than a browser.
*/
func trusted(leaves []audience.LeafResult, c models.Contact) []audience.LeafResult {
	out := make([]audience.LeafResult, 0, len(leaves))
	for _, leaf := range leaves {
		key, custom := strings.CutPrefix(leaf.Field, "contact.custom.")
		if custom && slices.Contains(c.TrustedAttributeKeys, key) {
			leaf.Trust = audience.Verified
		}

		out = append(out, leaf)
	}

	return out
}
