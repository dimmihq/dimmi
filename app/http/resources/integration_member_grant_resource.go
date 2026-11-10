/*
|--------------------------------------------------------------------------
| Integration Member Grant Resources
|--------------------------------------------------------------------------
|
| A teammate's own grant on an installation, without its token, and
| where to send a browser to approve an install or a grant.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
IntegrationMemberGrant is a teammate's own grant, without its token.
*/
type IntegrationMemberGrant struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	MemberID       string     `json:"member_id"`
	ExternalUserID string     `json:"external_user_id"`
	Scopes         []string   `json:"scopes"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
}

/*
NewIntegrationMemberGrant shapes a teammate's grant.
*/
func NewIntegrationMemberGrant(g models.IntegrationMemberGrant) IntegrationMemberGrant {
	return IntegrationMemberGrant{
		ID: models.PublicID(models.PrefixIntegrationMemberGrant, g.ID), Object: "integration_member_grant",
		MemberID: models.PublicID(models.PrefixMember, g.MemberID), ExternalUserID: g.ExternalUserID,
		Scopes: nonNilList(g.Scopes), Status: g.Status, CreatedAt: g.CreatedAt, RevokedAt: g.RevokedAt,
	}
}

/*
IntegrationAuthorization is where to send the browser to approve.
*/
type IntegrationAuthorization struct {
	Object       string `json:"object"`
	AuthorizeURL string `json:"authorize_url"`
}
