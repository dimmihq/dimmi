/*
|--------------------------------------------------------------------------
| SES Identities
|--------------------------------------------------------------------------
|
| SES signs mail for a domain only once it knows that domain as an identity
| and has seen its signing records, so a caller creates the identity first
| and shows the customer the tokens it returns. Keys are always RSA 2048.
| A missing bounce record never rejects mail: SES falls back to its own
| return path, because a slower setup should not stop a reply.
|
| One customer's complaints can stay theirs through a tenant, which groups
| their identities and pauses them apart from everyone else's. The account
| level suppression list is shared, so removing an address from it is a
| separate call a caller only makes for what it may lift.
|
*/

package ses

import (
	"context"
	"errors"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
ErrIdentityNotFound means SES has no such identity. It also matches
mail.ErrPermanent.
*/
var ErrIdentityNotFound = fmt.Errorf("ses: identity not found: %w", mail.ErrPermanent)

/*
Identity is a sending domain as SES sees it. Tokens are the signing tokens
whose aliases point into SigningHostedZone. DKIMStatus and MailFromStatus
are what SES last found, in its own words, such as SUCCESS or PENDING.
*/
type Identity struct {
	Tokens            []string
	SigningHostedZone string
	DKIMStatus        string
	MailFromStatus    string
	Verified          bool
}

/*
Identities manages SES sending identities.
*/
type Identities struct {
	client *sesv2.Client
	region string
}

/*
NewIdentities returns the manager on the AWS configuration.
*/
func NewIdentities(cfg awssdk.Config) *Identities {
	client := sesv2.NewFromConfig(cfg, func(o *sesv2.Options) {
		o.Retryer = awssdk.NopRetryer{}
	})

	return &Identities{client: client, region: cfg.Region}
}

/*
Region returns the region identities live in.
*/
func (i *Identities) Region() string {
	return i.region
}

/*
Create registers the domain, or returns it as it is when another caller
already registered it: an identity is unique per account and region.
*/
func (i *Identities) Create(ctx context.Context, domain string) (Identity, error) {
	_, err := i.client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: awssdk.String(domain),
		DkimSigningAttributes: &types.DkimSigningAttributes{
			NextSigningKeyLength: types.DkimSigningKeyLengthRsa2048Bit,
		},
	})
	if err != nil && !isCode(err, "AlreadyExistsException") {
		return Identity{}, classifyAs("create identity", err)
	}

	return i.Get(ctx, domain)
}

/*
Get returns the identity's current state.
*/
func (i *Identities) Get(ctx context.Context, domain string) (Identity, error) {
	out, err := i.client.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{EmailIdentity: awssdk.String(domain)})
	if isCode(err, "NotFoundException") {
		return Identity{}, ErrIdentityNotFound
	}

	if err != nil {
		return Identity{}, classifyAs("get identity", err)
	}

	identity := Identity{Verified: out.VerifiedForSendingStatus}
	if out.DkimAttributes != nil {
		identity.Tokens = out.DkimAttributes.Tokens
		identity.SigningHostedZone = awssdk.ToString(out.DkimAttributes.SigningHostedZone)
		identity.DKIMStatus = string(out.DkimAttributes.Status)
	}

	if out.MailFromAttributes != nil {
		identity.MailFromStatus = string(out.MailFromAttributes.MailFromDomainStatus)
	}

	return identity, nil
}

/*
SetMailFrom points the identity's return path at the name, falling back to
SES's own when its record is missing.
*/
func (i *Identities) SetMailFrom(ctx context.Context, domain, mailFrom string) error {
	_, err := i.client.PutEmailIdentityMailFromAttributes(ctx, &sesv2.PutEmailIdentityMailFromAttributesInput{
		EmailIdentity:       awssdk.String(domain),
		MailFromDomain:      awssdk.String(mailFrom),
		BehaviorOnMxFailure: types.BehaviorOnMxFailureUseDefaultValue,
	})
	if err != nil {
		return classifyAs("set mail from", err)
	}

	return nil
}

/*
RestartDKIM makes SES look for the signing records again, after it gave up
on them.
*/
func (i *Identities) RestartDKIM(ctx context.Context, domain string) error {
	_, err := i.client.PutEmailIdentityDkimSigningAttributes(ctx, &sesv2.PutEmailIdentityDkimSigningAttributesInput{
		EmailIdentity:           awssdk.String(domain),
		SigningAttributesOrigin: types.DkimSigningAttributesOriginAwsSes,
		SigningAttributes: &types.DkimSigningAttributes{
			NextSigningKeyLength: types.DkimSigningKeyLengthRsa2048Bit,
		},
	})
	if err != nil {
		return classifyAs("restart dkim", err)
	}

	return nil
}

/*
Delete removes the identity. One already gone is no failure.
*/
func (i *Identities) Delete(ctx context.Context, domain string) error {
	_, err := i.client.DeleteEmailIdentity(ctx, &sesv2.DeleteEmailIdentityInput{EmailIdentity: awssdk.String(domain)})
	if err != nil && !isCode(err, "NotFoundException") {
		return classifyAs("delete identity", err)
	}

	return nil
}

/*
AssociateTenant makes sure the tenant exists and holds the resource, named
by its full ARN. Either being there already is no failure.
*/
func (i *Identities) AssociateTenant(ctx context.Context, tenant, resourceARN string) error {
	_, err := i.client.CreateTenant(ctx, &sesv2.CreateTenantInput{TenantName: awssdk.String(tenant)})
	if err != nil && !isCode(err, "AlreadyExistsException") {
		return classifyAs("create tenant", err)
	}

	_, err = i.client.CreateTenantResourceAssociation(ctx, &sesv2.CreateTenantResourceAssociationInput{
		TenantName: awssdk.String(tenant), ResourceArn: awssdk.String(resourceARN),
	})
	if err != nil && !isCode(err, "AlreadyExistsException") {
		return classifyAs("associate tenant", err)
	}

	return nil
}

/*
RemoveSuppressed lifts the address from the account's suppression list. An
address not on it is no failure.
*/
func (i *Identities) RemoveSuppressed(ctx context.Context, email string) error {
	_, err := i.client.DeleteSuppressedDestination(ctx, &sesv2.DeleteSuppressedDestinationInput{
		EmailAddress: awssdk.String(email),
	})
	if err != nil && !isCode(err, "NotFoundException") {
		return classifyAs("remove suppressed address", err)
	}

	return nil
}

/*
isCode reports whether SES answered with the error code.
*/
func isCode(err error, code string) bool {
	var apiErr smithy.APIError

	return errors.As(err, &apiErr) && apiErr.ErrorCode() == code
}

/*
classifyAs maps an SES error to the mail error kinds, saying what was being
attempted.
*/
func classifyAs(attempt string, err error) error {
	return fmt.Errorf("ses %s: %w", attempt, classify(err))
}
