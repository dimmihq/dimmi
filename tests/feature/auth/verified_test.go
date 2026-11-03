package auth_test

import (
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
gatedStep is a route behind the verified middleware, standing in for
the steps that reach other people, such as inviting them.
*/
func gatedStep(r *routing.Router) {
	r.Group("/testing", func(g *routing.Router) {
		g.Middleware("session", "auth", "verified")
		done := func(w fhttp.ResponseWriter, _ *fhttp.Request) { w.WriteHeader(fhttp.StatusNoContent) }
		g.Handle(fhttp.MethodPost, "/gated", "testing.gated", fhttp.HandlerFunc(done))
	})
}

func TestVerifiedMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("holds back a user whose email is unproven where the install requires it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithRoutes(gatedStep), tests.WithEnv("DIMMI_AUTH__VERIFICATION__REQUIRED=true"))

		tc.ActingAs(tc.CreateUser(factories.Unverified)).PostJSON("/testing/gated", nil).
			AssertForbidden().
			AssertJSONPath("code", "email_unverified")
		tc.ActingAsOwner().PostJSON("/testing/gated", nil).AssertNoContent()
	})

	t.Run("requires a proven email on the cloud but not on a self-hosted install", func(t *testing.T) {
		t.Parallel()
		selfHosted := tests.New(t, tests.WithRoutes(gatedStep))
		cloud := tests.New(t, tests.WithRoutes(gatedStep),
			tests.WithEnv("DIMMI_APP__EDITION=cloud", "DIMMI_APP__REGION=us"))

		selfHosted.ActingAs(selfHosted.CreateUser(factories.Unverified)).PostJSON("/testing/gated", nil).
			AssertNoContent()
		cloud.ActingAs(cloud.CreateUser(factories.Unverified)).PostJSON("/testing/gated", nil).AssertForbidden()
	})
}
