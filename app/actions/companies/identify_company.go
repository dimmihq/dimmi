/*
|--------------------------------------------------------------------------
| Identify Company
|--------------------------------------------------------------------------
|
| A developer's server names the company a customer works for, by the
| host app's own id or by its web domain, and it is found or added. A
| domain is stored lower case without www, so acme.dev and WWW.Acme.dev
| are one company. Details sent replace what is stored; details left out
| stay.
|
*/

package companies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
Fields are a company's details as a request gives them, nil when left
out.
*/
type Fields struct {
	ExternalID       *string
	Name             *string
	Domain           *string
	CustomAttributes map[string]any
}

/*
NormalizeDomain returns the domain in lower case without www, nil when
empty.
*/
func NormalizeDomain(domain *string) *string {
	if domain == nil {
		return nil
	}

	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(*domain)), "www.")
	if d == "" {
		return nil
	}

	return &d
}

/*
Identity is the company a trusted caller names, in the workspace, at a
moment.
*/
type Identity struct {
	WorkspaceID ids.UUID
	Fields      Fields
	Now         time.Time
}

/*
Identify finds the company by external id, then domain, adding one when
there is none. It reports whether it added one.
*/
func Identify(ctx context.Context, tx *database.Tx, i Identity) (models.Company, bool, error) {
	q, f := models.New(tx), i.Fields
	domain := NormalizeDomain(f.Domain)

	if f.ExternalID == nil && domain == nil {
		return models.Company{}, false, exceptions.ErrUpsertKeyRequired
	}

	found, matched, err := match(ctx, q, i.WorkspaceID, f)
	if err != nil {
		return found, false, err
	}

	if !matched {
		c, err := create(ctx, q, i)

		return c, true, err
	}

	merged, _, err := attributes.Merge(found.CustomAttributes, f.CustomAttributes)
	if err != nil {
		return found, false, err
	}

	params := paramsOf(found, i.Now)
	params.CustomAttributes = merged

	if f.Name != nil {
		params.Name = *f.Name
	}

	if domain != nil {
		params.Domain = domain
	}

	if f.ExternalID != nil {
		params.ExternalID = f.ExternalID
	}

	c, err := q.UpdateCompany(ctx, params)
	if err != nil {
		return found, false, fmt.Errorf("update company: %w", err)
	}

	return c, false, nil
}

/*
match finds the live company with the external id, then the domain.
*/
func match(ctx context.Context, q *models.Queries, ws ids.UUID, f Fields) (models.Company, bool, error) {
	if f.ExternalID != nil {
		c, err := q.FindCompanyByExternalID(ctx, models.FindCompanyByExternalIDParams{
			WorkspaceID: ws, ExternalID: f.ExternalID,
		})
		if err == nil {
			return c, true, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return c, false, fmt.Errorf("find company: %w", err)
		}
	}

	domain := NormalizeDomain(f.Domain)
	if domain == nil {
		return models.Company{}, false, nil
	}

	c, err := q.FindCompanyByDomain(ctx, models.FindCompanyByDomainParams{WorkspaceID: ws, Domain: domain})
	if errors.Is(err, database.ErrNotFound) {
		return c, false, nil
	}

	if err != nil {
		return c, false, fmt.Errorf("find company: %w", err)
	}

	return c, true, nil
}

/*
create adds the company, naming it after its domain or external id when
no name was given.
*/
func create(ctx context.Context, q *models.Queries, i Identity) (models.Company, error) {
	f := i.Fields

	attrs, _, err := attributes.Merge(nil, f.CustomAttributes)
	if err != nil {
		return models.Company{}, err
	}

	c, err := q.CreateCompany(ctx, models.CreateCompanyParams{
		WorkspaceID: i.WorkspaceID, ExternalID: f.ExternalID, Name: displayName(f),
		Domain: NormalizeDomain(f.Domain), CustomAttributes: attrs, Now: i.Now,
	})
	if database.IsUniqueViolation(err, externalIDConstraint) {
		return c, exceptions.ErrCompanyExists
	}

	if err != nil {
		return c, fmt.Errorf("create company: %w", err)
	}

	return c, nil
}

/*
displayName returns what to call a new company: its name, else its
domain, else its external id.
*/
func displayName(f Fields) string {
	for _, candidate := range []*string{f.Name, NormalizeDomain(f.Domain), f.ExternalID} {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			return *candidate
		}
	}

	return ""
}

/*
IdentifyInput is who names which company.
*/
type IdentifyInput struct {
	Membership authz.Membership
	Fields     Fields
}

/*
IdentifyCompany finds or adds the company a trusted caller names.

Rules:
  - The caller needs contacts:write.
  - An external id or a domain is required.

Events: [appevents.CompanyCreated] or [appevents.CompanyUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrUpsertKeyRequired],
[exceptions.ErrInvalidAttributes], or a wrapped database error.
*/
type IdentifyCompany struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle returns the company and whether it was added.
*/
func (a *IdentifyCompany) Handle(ctx context.Context, in IdentifyInput) (Detail, bool, error) {
	if !in.Membership.Can("contacts:write") {
		return Detail{}, false, exceptions.ErrForbidden
	}

	var (
		out     Detail
		created bool
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		ws := in.Membership.Workspace.ID

		c, added, err := Identify(ctx, tx, Identity{WorkspaceID: ws, Fields: in.Fields, Now: a.Clock.Now()})
		if err != nil {
			return err
		}

		created = added

		if out, err = LoadDetail(ctx, models.New(tx), c); err != nil {
			return err
		}

		var event any = appevents.CompanyUpdated{WorkspaceID: ws, CompanyID: c.ID, ActorID: in.Membership.Member.ID}
		if added {
			event = appevents.CompanyCreated{WorkspaceID: ws, CompanyID: c.ID, ActorID: in.Membership.Member.ID}
		}

		return a.Events.Dispatch(ctx, tx, event)
	})

	return out, created, err
}
