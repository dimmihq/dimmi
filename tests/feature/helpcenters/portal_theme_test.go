package helpcenters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestPortalTheme(t *testing.T) {
	t.Parallel()

	t.Run("offers readers the theme switch while the team leaves the choice to them", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)

		page := string(c.page("").AssertOK().Body)

		assert.Contains(t, page, `data-theme-switch`)
		assert.Contains(t, page, `content="light dark"`)
	})

	t.Run("gives every reader the team's theme and no switch once the team picks one", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"settings": testkit.JSON{"appearance": "dark"}}).
			AssertOK()

		page := string(c.page("").AssertOK().Body)

		assert.Contains(t, page, `data-appearance="dark"`)
		assert.NotContains(t, page, `data-theme-switch`)
	})
}
