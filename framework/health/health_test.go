package health_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/health"
)

/*
returning is a check that always gives the result.
*/
func returning(name string, result health.Result) health.Check {
	return health.Func(name, func(context.Context) health.Result { return result })
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("passes with no checks", func(t *testing.T) {
		t.Parallel()

		report := health.NewRegistry(0, clock.System()).Run(t.Context())

		assert.Equal(t, health.Pass, report.Status)
		assert.Empty(t, report.Outcomes)
		assert.Empty(t, report.Failed())
	})

	t.Run("reports each check in the order added", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(
			returning("database", health.Result{Status: health.Pass, Detail: "reachable"}),
			returning("mail", health.Result{Status: health.Skip}),
		)

		report := registry.Run(t.Context())

		require.Len(t, report.Outcomes, 2)
		assert.Equal(t, health.Pass, report.Status)
		assert.Equal(t, "database", report.Outcomes[0].Name)
		assert.Equal(t, health.Result{Status: health.Pass, Detail: "reachable"}, report.Outcomes[0].Result)
		assert.Equal(t, "mail", report.Outcomes[1].Name)
	})

	t.Run("warns when a check warns and none fails", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(
			returning("a", health.Result{Status: health.Pass}),
			returning("b", health.Result{Status: health.Warn}),
		)

		assert.Equal(t, health.Warn, registry.Run(t.Context()).Status)
	})

	t.Run("fails when any check fails and names it", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(
			returning("a", health.Result{Status: health.Warn}),
			returning("b", health.Result{Status: health.Fail, Detail: "down", Fix: "start it"}),
		)

		report := registry.Run(t.Context())

		assert.Equal(t, health.Fail, report.Status)
		assert.Equal(t, []string{"b"}, report.Failed())
	})

	t.Run("replaces a check added again under the same name", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(
			returning("a", health.Result{Status: health.Fail}),
			returning("b", health.Result{Status: health.Pass}),
		)
		registry.Add(returning("a", health.Result{Status: health.Pass}))

		report := registry.Run(t.Context())

		assert.Equal(t, health.Pass, report.Status)
		assert.Equal(t, "a", report.Outcomes[0].Name)
	})

	t.Run("fails a check that does not answer within the timeout", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		registry := health.NewRegistry(20*time.Millisecond, clock.System())
		registry.Add(health.Func("stuck", func(context.Context) health.Result {
			<-release

			return health.Result{Status: health.Pass}
		}))

		report := registry.Run(t.Context())

		assert.Equal(t, health.Fail, report.Status)
		assert.Equal(t, "no answer within 20ms", report.Outcomes[0].Result.Detail)
	})

	t.Run("hands each check a context that ends at the timeout", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(20*time.Millisecond, clock.System())
		registry.Add(health.Func("patient", func(ctx context.Context) health.Result {
			<-ctx.Done()

			return health.Result{Status: health.Fail, Detail: ctx.Err().Error()}
		}))

		report := registry.Run(t.Context())

		assert.Equal(t, health.Fail, report.Status)
	})

	t.Run("fails a check that panics", func(t *testing.T) {
		t.Parallel()
		registry := health.NewRegistry(time.Second, clock.System())
		registry.Add(health.Func("broken", nil))

		report := registry.Run(t.Context())

		assert.Equal(t, health.Fail, report.Status)
		assert.Contains(t, report.Outcomes[0].Result.Detail, "check panicked")
	})

	t.Run("measures how long each check ran", func(t *testing.T) {
		t.Parallel()
		fake := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
		registry := health.NewRegistry(time.Second, fake)
		registry.Add(health.Func("slow", func(context.Context) health.Result {
			fake.Travel(150 * time.Millisecond)

			return health.Result{Status: health.Pass}
		}))

		report := registry.Run(t.Context())

		assert.Equal(t, 150*time.Millisecond, report.Outcomes[0].Duration)
	})
}
