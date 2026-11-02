package auth_test

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/database/factories"
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
