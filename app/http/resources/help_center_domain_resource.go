/*
|--------------------------------------------------------------------------
| Help Center Domain Resources
|--------------------------------------------------------------------------
|
| How a help center's custom domain reads over the API, with the two
| records the team creates and where its check stands.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DNSRecord is one record the team creates at its DNS provider.
*/
type DNSRecord struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

/*
HelpCenterDomain is a help center's custom domain.
*/
type HelpCenterDomain struct {
	Object        string      `json:"object"`
	HelpCenterID  string      `json:"help_center_id"`
	Hostname      string      `json:"hostname"`
	Status        string      `json:"status"`
	FailureReason *string     `json:"failure_reason"`
	Records       []DNSRecord `json:"records"`
	LastCheckedAt *time.Time  `json:"last_checked_at"`
	NextCheckAt   *time.Time  `json:"next_check_at"`
	VerifiedAt    *time.Time  `json:"verified_at"`
	CreatedAt     time.Time   `json:"created_at"`
}

/*
NewHelpCenterDomain shapes a help center's custom domain.
*/
func NewHelpCenterDomain(d helpcenters.Domain) HelpCenterDomain {
	return HelpCenterDomain{
		Object:       "help_center_domain",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, d.HelpCenterID), Hostname: d.Hostname,
		Status: d.Status, FailureReason: d.FailureReason,
		Records: []DNSRecord{
			{Type: "CNAME", Name: d.CNAMEName, Value: d.Target},
			{Type: "TXT", Name: d.TXTName, Value: d.TXTValue},
		},
		LastCheckedAt: d.LastCheckedAt, NextCheckAt: d.NextCheckAt, VerifiedAt: d.VerifiedAt, CreatedAt: d.CreatedAt,
	}
}
