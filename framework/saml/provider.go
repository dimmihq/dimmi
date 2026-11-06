/*
|--------------------------------------------------------------------------
| SAML Service Provider
|--------------------------------------------------------------------------
|
| Signs people in through an identity provider by the browser profile:
| a signed request sent by redirect, a signed answer posted back. The
| library checks signatures; everything it only warns about, or does not
| check at all, is checked here and refused: a wrong audience, a stale
| assertion, a wrong destination, an answer to another request, more than
| one assertion. An answer nobody asked for is told apart so the caller
| can start a proper sign-in instead of trusting it. Library errors never
| leave this package; callers get one of the errors below.
|
*/

package saml

import (
	"crypto"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
The ways an answer from an identity provider is refused.

ErrInvalidSignature means no signature by a listed certificate covers
the assertion. ErrExpired means it is outside its validity window.
ErrWrongAudience, ErrWrongDestination and ErrWrongIssuer mean it was
meant for, sent to or made by someone else. ErrInResponseTo means it
answers another request, ErrUnsolicited that it answers none while one
was not expected either. ErrMalformed covers everything that does not
read, is too large or holds no single assertion with a name, and
ErrStatusNotSuccess an identity provider that said no.
*/
var (
	ErrInvalidSignature = errors.New("saml: invalid signature")
	ErrExpired          = errors.New("saml: assertion expired or not yet valid")
	ErrWrongAudience    = errors.New("saml: wrong audience")
	ErrWrongDestination = errors.New("saml: wrong destination")
	ErrWrongIssuer      = errors.New("saml: wrong issuer")
	ErrInResponseTo     = errors.New("saml: answers another request")
	ErrUnsolicited      = errors.New("saml: unsolicited response")
	ErrMalformed        = errors.New("saml: malformed response")
	ErrStatusNotSuccess = errors.New("saml: identity provider refused")
)

/*
Limits on what is read from an identity provider.
*/
const (
	maxEncodedResponse = 256 * 1024
	maxXMLTokens       = 100_000
	maxDecompressed    = 1 << 20
	defaultClockSkew   = 3 * time.Minute
)

/*
IdentityProvider is the identity provider side: its entity id, the
address sign-ins are sent to, and the certificates its signatures may
come from, several during a rollover.
*/
type IdentityProvider struct {
	EntityID     string
	SSOURL       string
	Certificates []*x509.Certificate
}

/*
ServiceProvider is this side: its entity id, the address answers are
posted to, and its own key pair.
*/
type ServiceProvider struct {
	EntityID    string
	ACSURL      string
	Certificate *x509.Certificate
	PrivateKey  crypto.Signer
}

/*
Config is everything a sign-in needs. ClockSkew is how far the two
sides' clocks may disagree, three minutes when zero.
*/
type Config struct {
	SP           ServiceProvider
	IDP          IdentityProvider
	NameIDFormat string
	Clock        clock.Clock
	ClockSkew    time.Duration
}

/*
Assertion is what an accepted answer says about the person.
*/
type Assertion struct {
	AssertionID  string
	ResponseID   string
	NameID       string
	NameIDFormat string
	Attributes   map[string][]string
	AuthnInstant time.Time
	NotOnOrAfter time.Time
}

/*
Provider signs people in through one identity provider.
*/
type Provider struct {
	cfg Config
}

/*
New returns a provider for the configuration, refusing one without the
keys or addresses a sign-in needs.
*/
func New(cfg Config) (*Provider, error) {
	if cfg.SP.PrivateKey == nil || cfg.SP.Certificate == nil || cfg.SP.EntityID == "" || cfg.SP.ACSURL == "" {
		return nil, fmt.Errorf("%w: service provider incomplete", ErrInvalidKey)
	}

	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = defaultClockSkew
	}

	return &Provider{cfg: cfg}, nil
}

/*
library returns the library's service provider. Its clock runs the skew
behind, so the library's own expiry checks allow for it too; this
package checks both ends of the window itself. The key stores only
refuse a missing signer, which New already refused.
*/
func (p *Provider) library() *saml2.SAMLServiceProvider {
	store := dsig.MemoryX509CertificateStore{Roots: p.cfg.IDP.Certificates}

	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      p.cfg.IDP.SSOURL,
		IdentityProviderSSOBinding:  saml2.BindingHttpRedirect,
		IdentityProviderIssuer:      p.cfg.IDP.EntityID,
		AssertionConsumerServiceURL: p.cfg.SP.ACSURL,
		ServiceProviderIssuer:       p.cfg.SP.EntityID,
		AudienceURI:                 p.cfg.SP.EntityID,
		SignAuthnRequests:           true,
		SignAuthnRequestsAlgorithm:  dsig.RSASHA256SignatureMethod,
		IDPCertificateStore:         &store,
		NameIdFormat:                p.cfg.NameIDFormat,
		AllowMissingAttributes:      true,
		Clock:                       dsig.NewFakeClockAt(p.cfg.Clock.Now().Add(-p.cfg.ClockSkew)),
		MaximumDecompressedBodySize: maxDecompressed,
		MaximumXMLTokens:            maxXMLTokens,
	}

	keys := &saml2.KeyStore{Signer: p.cfg.SP.PrivateKey, Cert: p.cfg.SP.Certificate.Raw}
	_ = sp.SetSPKeyStore(keys)
	_ = sp.SetSPSigningKeyStore(keys)

	return sp
}

/*
AuthURL returns the address that starts a sign-in at the identity
provider, carrying the relay state, and the id of the request it sends,
which the answer must name.
*/
func (p *Provider) AuthURL(relayState string) (string, string, error) {
	sp := p.library()

	doc, err := sp.BuildAuthRequestDocument()
	if err != nil {
		return "", "", fmt.Errorf("build sign-in request: %w", err)
	}

	requestID := doc.Root().SelectAttrValue("ID", "")

	address, err := sp.BuildAuthURLRedirect(relayState, doc)
	if err != nil {
		return "", "", fmt.Errorf("build sign-in address: %w", err)
	}

	return address, requestID, nil
}

/*
Metadata returns this side's metadata document, publishing its
certificate for signing and encryption.
*/
func (p *Provider) Metadata() ([]byte, error) {
	descriptor, err := p.library().Metadata()
	if err != nil {
		return nil, fmt.Errorf("build metadata: %w", err)
	}

	publishSigning(descriptor.SPSSODescriptor)

	out, err := xml.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}

	return append([]byte(xml.Header), out...), nil
}

/*
publishSigning adds the signing use of the service provider's
certificate, which the library leaves out when the key is set the way it
recommends. The one key pair both signs and decrypts.
*/
func publishSigning(sp *types.SPSSODescriptor) {
	for _, key := range sp.KeyDescriptors {
		if key.Use == "signing" {
			return
		}
	}

	for _, key := range sp.KeyDescriptors {
		if key.Use == "encryption" {
			sp.KeyDescriptors = append(sp.KeyDescriptors, types.KeyDescriptor{Use: "signing", KeyInfo: key.KeyInfo})

			return
		}
	}
}

/*
Consume checks an answer posted back and returns what it says. With an
empty expectedRequestID, an otherwise valid answer that names no request
is ErrUnsolicited.
*/
func (p *Provider) Consume(encodedResponse, expectedRequestID string) (Assertion, error) {
	if len(encodedResponse) > maxEncodedResponse {
		return Assertion{}, fmt.Errorf("%w: too large", ErrMalformed)
	}

	sp := p.library()

	response, err := sp.ValidateEncodedResponse(encodedResponse)
	if err != nil {
		return Assertion{}, classify(err)
	}

	if len(response.Assertions) != 1 || len(response.EncryptedAssertions) > 0 {
		return Assertion{}, fmt.Errorf("%w: expected one assertion", ErrMalformed)
	}

	assertion := response.Assertions[0]

	if err := p.check(sp, response, &assertion); err != nil {
		return Assertion{}, err
	}

	out, err := read(response, assertion)
	if err != nil {
		return Assertion{}, err
	}

	return out, p.answers(response, assertion, expectedRequestID)
}

/*
check refuses an answer sent somewhere else, outside its validity window
or meant for another audience. The library has checked the subject's
confirmation: its method, recipient and expiry.
*/
func (p *Provider) check(sp *saml2.SAMLServiceProvider, response *types.Response, a *types.Assertion) error {
	if response.Destination != p.cfg.SP.ACSURL {
		return ErrWrongDestination
	}

	warnings, err := sp.VerifyAssertionConditions(a)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}

	if warnings.NotInAudience || len(a.Conditions.AudienceRestrictions) == 0 {
		return ErrWrongAudience
	}

	return p.within(a.Conditions.NotBefore, a.Conditions.NotOnOrAfter)
}

/*
within refuses a window the current time, give or take the skew, is not
in. The library already refused times that do not parse.
*/
func (p *Provider) within(notBefore, notOnOrAfter string) error {
	now := p.cfg.Clock.Now()
	start, _ := time.Parse(time.RFC3339, notBefore)
	end, _ := time.Parse(time.RFC3339, notOnOrAfter)

	if now.Add(p.cfg.ClockSkew).Before(start) || !now.Add(-p.cfg.ClockSkew).Before(end) {
		return ErrExpired
	}

	return nil
}

/*
answers refuses an answer to another request, and reports one to no
request.
*/
func (p *Provider) answers(response *types.Response, a types.Assertion, expected string) error {
	inResponseTo := response.InResponseTo

	if data := confirmationData(&a); data != nil && data.InResponseTo != "" && data.InResponseTo != inResponseTo {
		return ErrInResponseTo
	}

	switch {
	case expected == "" && inResponseTo == "":
		return ErrUnsolicited
	case inResponseTo != expected:
		return ErrInResponseTo
	default:
		return nil
	}
}

/*
confirmationData returns the data of the assertion's subject
confirmation, which the library made sure is there.
*/
func confirmationData(a *types.Assertion) *types.SubjectConfirmationData {
	return a.Subject.SubjectConfirmation.SubjectConfirmationData
}

/*
read returns what the assertion says, refusing one without a name.
*/
func read(response *types.Response, a types.Assertion) (Assertion, error) {
	if a.Subject.NameID == nil || a.Subject.NameID.Value == "" {
		return Assertion{}, fmt.Errorf("%w: no name id", ErrMalformed)
	}

	out := Assertion{
		AssertionID: a.ID, ResponseID: response.ID, NameID: a.Subject.NameID.Value,
		NameIDFormat: a.Subject.NameID.Format, Attributes: map[string][]string{},
	}

	if a.AttributeStatement != nil {
		for _, attribute := range a.AttributeStatement.Attributes {
			for _, value := range attribute.Values {
				out.Attributes[attribute.Name] = append(out.Attributes[attribute.Name], value.Value)
			}
		}
	}

	if a.AuthnStatement != nil && a.AuthnStatement.AuthnInstant != nil {
		out.AuthnInstant = *a.AuthnStatement.AuthnInstant
	}

	out.NotOnOrAfter, _ = time.Parse(time.RFC3339, a.Conditions.NotOnOrAfter)

	return out, nil
}

/*
classify turns a library error into one of this package's.
*/
func classify(err error) error {
	if invalid, ok := errors.AsType[saml2.ErrInvalidValue](err); ok {
		switch invalid.Key {
		case saml2.IssuerTag:
			return ErrWrongIssuer
		case saml2.StatusCodeTag:
			return ErrStatusNotSuccess
		case saml2.DestinationAttr, saml2.RecipientAttr:
			return ErrWrongDestination
		case saml2.NotOnOrAfterAttr:
			return ErrExpired
		}
	}

	if missing, ok := errors.AsType[saml2.ErrMissingElement](err); ok {
		return fmt.Errorf("%w: missing %s", ErrMalformed, missing.Tag)
	}

	if errors.Is(err, dsig.ErrMissingSignature) || isSignatureError(err) {
		return fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}

	return fmt.Errorf("%w: %w", ErrMalformed, err)
}

/*
isSignatureError reports whether the library refused the answer over
its signature. The library words most of these errors without a type, so
their text is all there is to go on.
*/
func isSignatureError(err error) bool {
	if errors.Is(err, dsig.ErrInvalidSignature) {
		return true
	}

	text := strings.ToLower(err.Error())

	return strings.Contains(text, "signature") || strings.Contains(text, "digest") ||
		strings.Contains(text, "certificate") || strings.Contains(text, "unsigned")
}
