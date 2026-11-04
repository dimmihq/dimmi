/*
|--------------------------------------------------------------------------
| Workspace And Invitation Settings
|--------------------------------------------------------------------------
|
| How long a deleted workspace waits in the trash before it is purged,
| and how invitations to a workspace behave: how long a link stays good
| and how many a workspace may send in a day. The daily cap slows a
| workspace that would use invitations to send mail to strangers; a
| self-hosted install has no cap unless its operator sets one.
|
*/

package config

import "time"

/*
WorkspacesConfig sets DeletionGraceDays, the days a deleted workspace
can still be restored before it is purged.
*/
type WorkspacesConfig struct {
	DeletionGraceDays int `default:"30" koanf:"deletion_grace_days"`
}

/*
InvitationsConfig sets TTL, how long an invitation link works after it
was last sent, and MaxPerDay, the invitations one workspace may send in
a day, zero for no cap. MaxPerDay is settled from the edition when the
operator leaves it out.
*/
type InvitationsConfig struct {
	TTL       time.Duration `default:"168h" koanf:"ttl"`
	MaxPerDay *int          `koanf:"max_per_day"`
}

/*
settle fills the defaults that depend on the edition.
*/
func (c *InvitationsConfig) settle(edition Edition) {
	if c.MaxPerDay == nil {
		limit := map[bool]int{true: 100, false: 0}[edition == EditionCloud]
		c.MaxPerDay = &limit
	}
}

/*
DailyCap returns the invitations one workspace may send in a day, zero
for no cap.
*/
func (c *InvitationsConfig) DailyCap() int {
	if c.MaxPerDay == nil {
		return 0
	}

	return *c.MaxPerDay
}
