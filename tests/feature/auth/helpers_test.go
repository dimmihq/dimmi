package auth_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
registration is a valid sign-up body for the email.
*/
func registration(email string) testkit.JSON {
	return testkit.JSON{
		"name": "Grace Hopper", "email": email, "password": factories.Password, "workspace_name": "Navy",
	}
}

/*
login is a password sign-in body.
*/
func login(email, password string) testkit.JSON {
	return testkit.JSON{"grant": "password", "email": email, "password": password}
}

/*
ssoOnly is a login policy that allows only single sign-on, the way an
organization enforcing it will.
*/
type ssoOnly struct{}

func (ssoOnly) ForEmail(context.Context, string) (appauth.LoginPolicy, error) {
	return appauth.LoginPolicy{RedirectURL: "https://sso.example.com/start"}, nil
}

/*
brokenPolicy is a login policy that cannot be read, as when its store
is down.
*/
type brokenPolicy struct{}

func (brokenPolicy) ForEmail(context.Context, string) (appauth.LoginPolicy, error) {
	return appauth.LoginPolicy{}, errors.New("policy store unreachable")
}

/*
switchablePolicy allows every method until the test turns single
sign-on on, as when an organization starts enforcing it.
*/
type switchablePolicy struct {
	ssoOnly atomic.Bool
}

func (p *switchablePolicy) ForEmail(ctx context.Context, email string) (appauth.LoginPolicy, error) {
	if p.ssoOnly.Load() {
		return ssoOnly{}.ForEmail(ctx, email)
	}

	return appauth.LoginPolicy{}, nil
}

/*
membershipOf returns the user's only membership.
*/
func membershipOf(t *testing.T, tc *tests.TestCase, user models.User) models.WorkspaceMember {
	t.Helper()

	rows, err := models.New(tc.App.DB).ListUserWorkspaces(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	var member models.WorkspaceMember

	ctx := tenancy.With(t.Context(), rows[0].WorkspaceID.String())
	err = tc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		member, err = models.New(tx).FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: rows[0].WorkspaceID, ID: rows[0].MemberID,
		})

		return err
	})
	require.NoError(t, err)

	return member
}
