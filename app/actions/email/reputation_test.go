package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/config"
)

func TestHarmful(t *testing.T) {
	t.Parallel()

	rates := config.EmailReputation{BouncePauseRate: 0.05, ComplaintPauseRate: 0.001}

	cases := []struct {
		name   string
		counts email.SendingCounts
		rates  config.EmailReputation
		want   string
	}{
		{"too few emails to judge by", email.SendingCounts{Sent: 99, HardBounced: 99, Complained: 99}, rates, ""},
		{"at the bounce rate, not past it", email.SendingCounts{Sent: 100, HardBounced: 5}, rates, ""},
		{"past the bounce rate", email.SendingCounts{Sent: 100, HardBounced: 6}, rates, email.PausedBounceRate},
		{"at the complaint rate, not past it", email.SendingCounts{Sent: 1000, Complained: 1}, rates, ""},
		{"past the complaint rate", email.SendingCounts{Sent: 1000, Complained: 2}, rates, email.PausedComplaintRate},
		{
			"complaints come before bounces",
			email.SendingCounts{Sent: 100, HardBounced: 50, Complained: 1},
			rates,
			email.PausedComplaintRate,
		},
		{
			"rates turned off",
			email.SendingCounts{Sent: 100, HardBounced: 100, Complained: 100},
			config.EmailReputation{},
			"",
		},
		{
			"only the bounce rate set",
			email.SendingCounts{Sent: 100, HardBounced: 6, Complained: 50},
			config.EmailReputation{BouncePauseRate: 0.05},
			email.PausedBounceRate,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, email.Harmful(c.counts, c.rates))
		})
	}
}
