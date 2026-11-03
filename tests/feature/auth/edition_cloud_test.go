//go:build cloud

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestCloudEdition(t *testing.T) {
	t.Parallel()

	t.Run("reports the cloud's own defaults", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_APP__REGION=us", "DIMMI_AUTH__SIGNUP__MODE="))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.edition", "cloud").
			AssertJSONPath("data.region", "us").
			AssertJSONPath("data.signup_mode", "open")
	})

	t.Run("starts open to anyone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE="))

		tc.Client().PostJSON("/api/v1/users", registration("a@example.com")).AssertCreated()
	})

	t.Run("requires a proven email for steps that reach other people", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithRoutes(gatedStep), tests.WithEnv("DIMMI_APP__REGION=us"))

		tc.ActingAs(tc.CreateUser(factories.Unverified)).PostJSON("/testing/gated", nil).AssertForbidden()
	})

	t.Run("keeps every link out of the log", func(t *testing.T) {
		t.Parallel()
		logs := &syncBuffer{}
		tc := tests.New(t, tests.WithLogs(logs), tests.WithEnv(
			"DIMMI_LOG__LEVEL=info", "DIMMI_LOG__DRIVER=json", "DIMMI_APP__REGION=us",
		))

		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		tc.Drain()

		line := fallbackLine(t, logs)
		assert.Equal(t, "Reset your Dimmi password", line["subject"])
		assert.NotContains(t, line, "links")
		assert.NotContains(t, logs.String(), "reset-password#")
	})
}
