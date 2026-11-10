package broadcasts_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/config"
)

func TestBroadcastPacing(t *testing.T) {
	t.Parallel()

	t.Run("holds email back once the install's provider quota is spent", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t, "DIMMI_CHANNELS__EMAIL__SENDING__LIMITS__PROVIDER_PER_MINUTE=4",
			"DIMMI_CHANNELS__EMAIL__SENDING__LIMITS__BROADCAST_SHARE_PERCENT=100")
		for i := range 6 {
			d.contact(unique("quota"+string(rune('a'+i))), "Quota", "pro")
		}

		id := d.emailBroadcast("Quota", nil)
		d.send(id)

		report := d.report(id)
		report.AssertJSONPath("data.broadcast.status", "sending")
		report.AssertJSONPath("data.counts.sent", float64(4))
		report.AssertJSONPath("data.waiting.0.reason", "provider_limit")
		report.AssertJSONPath("data.waiting.0.count", float64(2))
	})

	t.Run("lets a new address send only its first day's allowance", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t, "DIMMI_BROADCASTS__WARMUP_FIRST_DAY=3")
		for i := range 5 {
			d.contact(unique("warm"+string(rune('a'+i))), "Warm", "pro")
		}

		id := d.emailBroadcast("Warm-up", nil)
		d.send(id)

		report := d.report(id)
		report.AssertJSONPath("data.counts.sent", float64(3))
		report.AssertJSONPath("data.waiting.0.reason", "warming_up")
		report.AssertJSONPath("data.waiting.0.count", float64(2))
	})

	t.Run("doubles the allowance each day until the address is trusted", func(t *testing.T) {
		t.Parallel()
		cfg := config.BroadcastsConfig{WarmupFirstDay: 500, WarmupUntil: 50000}
		day := 24 * time.Hour

		first, held := broadcasts.WarmupAllowance(cfg, time.Hour)
		assert.True(t, held)
		assert.Equal(t, 500, first)

		third, _ := broadcasts.WarmupAllowance(cfg, 2*day+time.Hour)
		assert.Equal(t, 2000, third)

		_, held = broadcasts.WarmupAllowance(cfg, 7*day)
		assert.False(t, held)
	})
}
