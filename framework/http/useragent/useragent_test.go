package useragent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/http/useragent"
)

func TestParse(t *testing.T) {
	t.Parallel()

	p := useragent.New()

	t.Run("reads a desktop browser", func(t *testing.T) {
		t.Parallel()
		got := p.Parse("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
			"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		assert.Equal(t, useragent.Agent{
			Browser: "Chrome", BrowserVersion: "120.0.0.0", OS: "MacOS", Device: useragent.Desktop,
		}, got)
	})

	t.Run("reads each kind of device", func(t *testing.T) {
		t.Parallel()
		cases := map[string]string{
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1": useragent.Mobile,
			"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1": useragent.Tablet,
			"Mozilla/5.0 (SMART-TV; Linux; Tizen 6.0) AppleWebKit/537.36 " +
				"(KHTML, like Gecko) SamsungBrowser/4.0 Chrome/76.0.3809.146 TV Safari/537.36": useragent.TV,
			"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)": useragent.Bot,
		}
		for ua, want := range cases {
			assert.Equal(t, want, p.Parse(ua).Device, ua)
		}
	})

	t.Run("gives an empty agent for an empty header", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, useragent.Agent{}, p.Parse(""))
	})

	t.Run("reports clients that are no browser as bots", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, useragent.Agent{Device: useragent.Bot}, p.Parse("curl/8.4.0"))
	})
}
