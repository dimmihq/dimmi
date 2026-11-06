/*
|--------------------------------------------------------------------------
| Organization Login Policy
|--------------------------------------------------------------------------
|
| A company that verified its domain and requires its identity provider
| decides how its people sign in: every address under that domain is
| sent to the identity provider, whatever way they tried. The answer is
| the same for an address with an account and one without, so nobody
| learns who has one. Addresses elsewhere sign in as usual.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OrganizationLoginPolicy reads the login policy from the domains
companies verified and the connections they route them to.
*/
type OrganizationLoginPolicy struct {
	DB *database.DB
}

/*
ForEmail returns the policy of the address's domain: sign in through
the enforced connection, or anything.
*/
func (p OrganizationLoginPolicy) ForEmail(ctx context.Context, email string) (LoginPolicy, error) {
	route, found, err := p.Route(ctx, email)
	if err != nil || !found || !route.Enforced {
		return LoginPolicy{}, err
	}

	return LoginPolicy{RedirectURL: route.RedirectURL}, nil
}

/*
SSORoute is the connection an address's domain routes to: its name,
where sign-in starts, whether the company requires it, and the company.
*/
type SSORoute struct {
	Name           string
	RedirectURL    string
	Enforced       bool
	OrganizationID ids.UUID
}

/*
Route returns the active connection the address's domain routes to,
when there is one.
*/
func (p OrganizationLoginPolicy) Route(ctx context.Context, email string) (SSORoute, bool, error) {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return SSORoute{}, false, nil
	}

	row, err := models.New(p.DB).FindSSOPolicyForDomain(ctx, strings.ToLower(strings.TrimSpace(email[at+1:])))
	if errors.Is(err, database.ErrNotFound) {
		return SSORoute{}, false, nil
	}

	if err != nil {
		return SSORoute{}, false, fmt.Errorf("find sso policy for domain: %w", err)
	}

	return SSORoute{
		Name: row.Name, Enforced: row.Enforced, OrganizationID: row.OrganizationID,
		RedirectURL: "/auth/sso/" + models.PublicID(models.PrefixSSOConnection, row.ID) + "/redirect",
	}, true, nil
}

/*
MayBypass reports whether the user is an active owner of the company
that requires its identity provider for the address, and so may sign in
around it while it is down, and the company.
*/
func (p OrganizationLoginPolicy) MayBypass(ctx context.Context, email string, userID ids.UUID) (ids.UUID, bool, error) {
	route, found, err := p.Route(ctx, email)
	if err != nil || !found || !route.Enforced {
		return ids.Nil(), false, err
	}

	workspace, err := models.New(p.DB).FindOrganizationWorkspace(ctx, route.OrganizationID)
	if errors.Is(err, database.ErrNotFound) {
		return ids.Nil(), false, nil
	}

	if err != nil {
		return ids.Nil(), false, fmt.Errorf("find company workspace: %w", err)
	}

	var owner bool

	err = p.DB.Transaction(tenancy.With(ctx, workspace.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		member, err := models.New(tx).FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
			WorkspaceID: workspace.ID, UserID: userID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		owner = err == nil && IsOwnerRole(enums.Role(member.Role)) && member.Status == "active"

		return err
	})
	if err != nil {
		return ids.Nil(), false, fmt.Errorf("check bypassing owner: %w", err)
	}

	return route.OrganizationID, owner, nil
}

/*
SSOBypassChecker decides whether a user may sign in around their
company's identity provider.
*/
type SSOBypassChecker interface {
	/*
		MayBypass reports whether the user may sign in around it, and the
		company whose owners hear about it.
	*/
	MayBypass(ctx context.Context, email string, userID ids.UUID) (ids.UUID, bool, error)
}

/*
Available reports whether any company on the install signs in through
its identity provider, so the sign-in page offers single sign-on.
*/
func (p OrganizationLoginPolicy) Available(ctx context.Context) (bool, error) {
	active, err := models.New(p.DB).AnySSOConnectionActive(ctx)
	if err != nil {
		return false, fmt.Errorf("check sso connections: %w", err)
	}

	return active, nil
}
