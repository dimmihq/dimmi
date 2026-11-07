/*
|--------------------------------------------------------------------------
| Identity Verification Resources
|--------------------------------------------------------------------------
|
| How a workspace's visitor sign-in reads over the API: its mode and its
| counts, which hold no per-person data.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IdentitySettings is a workspace's sign-in settings.
*/
type IdentitySettings struct {
	Object                   string     `json:"object"`
	Mode                     string     `json:"mode"`
	ClientWritableAttributes []string   `json:"client_writable_attributes"`
	UpdatedAt                *time.Time `json:"updated_at"`
}

/*
NewIdentitySettings shapes the settings.
*/
func NewIdentitySettings(s models.IdentitySetting) IdentitySettings {
	out := IdentitySettings{
		Object: "identity_settings", Mode: s.Mode, ClientWritableAttributes: s.ClientWritableAttributes,
	}
	if out.ClientWritableAttributes == nil {
		out.ClientWritableAttributes = []string{}
	}
	if !s.UpdatedAt.IsZero() {
		out.UpdatedAt = &s.UpdatedAt
	}
	return out
}

/*
IdentityStatsDay is one day's counts by outcome.
*/
type IdentityStatsDay struct {
	Day      string           `json:"day"`
	Outcomes map[string]int64 `json:"outcomes"`
}

/*
IdentityStats is the counts of the last days: totals by outcome, each
day's, and the share of starts that were signed in.
*/
type IdentityStats struct {
	Object        string             `json:"object"`
	Days          int                `json:"days"`
	Totals        map[string]int64   `json:"totals"`
	ByDay         []IdentityStatsDay `json:"by_day"`
	Verified      int64              `json:"verified"`
	Invalid       int64              `json:"invalid"`
	Anonymous     int64              `json:"anonymous"`
	VerifiedShare float64            `json:"verified_share"`
}

/*
NewIdentityStats shapes the counts.
*/
func NewIdentityStats(days int, s identity.Summary) IdentityStats {
	out := IdentityStats{
		Object: "identity_stats", Days: days, Totals: s.Totals, ByDay: make([]IdentityStatsDay, len(s.Days)),
		Verified: s.Verified, Invalid: s.Invalid, Anonymous: s.Anon, VerifiedShare: s.Share(),
	}
	for i, d := range s.Days {
		out.ByDay[i] = IdentityStatsDay{Day: d.Day, Outcomes: d.Outcomes}
	}
	return out
}
