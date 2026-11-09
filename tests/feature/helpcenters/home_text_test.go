package helpcenters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestHelpCenterHomeText(t *testing.T) {
	t.Parallel()

	t.Run("asks the default question and shows no footer until the team sets them", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)

		body := string(c.page("").Body)
		assert.Contains(t, body, "How can we help?")
		assert.NotContains(t, body, `>Contact us</a>`)
	})

	t.Run("shows the team's question, subtitle, footer and contact link", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"settings": testkit.JSON{
			"home_title": "What are you stuck on?", "home_subtitle": "Answers from the Acme team",
			"footer_text": "© Acme Inc.", "contact_url": "https://acme.example/contact",
		}}).AssertOK()

		home := string(c.page("").Body)
		assert.Contains(t, home, "What are you stuck on?")
		assert.NotContains(t, home, "How can we help?")
		assert.Contains(t, home, "Answers from the Acme team")
		for _, page := range []string{home, string(c.page("/search?q=x").Body)} {
			assert.Contains(t, page, "© Acme Inc.")
			assert.Contains(t, page, `href="https://acme.example/contact"`)
		}
	})
}
