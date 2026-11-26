package helpcenter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAgoReadsLikeAHelpCenter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	back := func(days int) time.Time { return now.AddDate(0, 0, -days) }
	for days, want := range map[int]string{
		0: "today", 1: "yesterday", 3: "3 days ago", 9: "over a week ago", 16: "over 2 weeks ago",
		45: "over a month ago", 70: "over 2 months ago", 400: "over a year ago", 800: "over 2 years ago",
	} {
		assert.Equal(t, want, Ago(back(days), now), days)
	}
}
