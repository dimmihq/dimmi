/*
|--------------------------------------------------------------------------
| Email Signature Resource
|--------------------------------------------------------------------------
|
| A signature as the settings pages show it: whose it is, which address
| it is for, its text as written and whether replies use it. A preview
| shows the same signature as it reads in a reply.
|
*/

package resources

import (
	"time"

	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailSignature is one signature. Kind is personal for a teammate's own
and address for the team's; MemberID is null for the team's, and
EmailAddressID null for one used from every address.
*/
type EmailSignature struct {
	ID             string    `json:"id"`
	Object         string    `json:"object"`
	Kind           string    `json:"kind"`
	MemberID       *string   `json:"member_id"`
	EmailAddressID *string   `json:"email_address_id"`
	Body           string    `json:"body"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

/*
EmailSignatureRendering is a signature as a reply shows it.
*/
type EmailSignatureRendering struct {
	Object string `json:"object"`
	HTML   string `json:"html"`
	Text   string `json:"text"`
}

/*
NewEmailSignature returns the resource of a signature.
*/
func NewEmailSignature(row models.EmailSignature) EmailSignature {
	out := EmailSignature{
		ID: models.PublicID(models.PrefixEmailSignature, row.ID), Object: "email_signature", Kind: "address",
		Body: row.Body, Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}

	if row.WorkspaceMemberID != nil {
		out.Kind = "personal"
		out.MemberID = new(models.PublicID(models.PrefixMember, *row.WorkspaceMemberID))
	}

	if row.EmailAddressID != nil {
		out.EmailAddressID = new(models.PublicID(models.PrefixEmailAddress, *row.EmailAddressID))
	}

	return out
}

/*
NewEmailSignatures returns the resources of signatures.
*/
func NewEmailSignatures(rows []models.EmailSignature) []EmailSignature {
	out := make([]EmailSignature, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewEmailSignature(row))
	}

	return out
}

/*
NewEmailSignatureRendering returns the resource of a rendered signature.
*/
func NewEmailSignatureRendering(s inbound.Signature) EmailSignatureRendering {
	return EmailSignatureRendering{Object: "email_signature_rendering", HTML: s.HTML, Text: s.Text}
}
