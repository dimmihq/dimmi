package arch_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit/arch"
)

/*
module is the module under test.
*/
const module = "github.com/dimmihq/dimmi"

func TestFrameworkBoundaries(t *testing.T) {
	t.Parallel()

	m := arch.Load(t, module)

	t.Run("framework never imports the application", func(t *testing.T) {
		t.Parallel()

		m.Expect("framework/...").
			NotToImport("app/...", "routes/...", "config/...", "bootstrap/...", "database/...", "cmd/...", "tests/...")
	})

	t.Run("only tests import the testkit", func(t *testing.T) {
		t.Parallel()

		m.Expect("framework/...").
			Except("framework/testkit/...", "framework/.../drivertest/...").
			IgnoringTests().
			NotToImport("framework/testkit/...")
	})
}
