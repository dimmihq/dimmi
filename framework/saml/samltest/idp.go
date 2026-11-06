/*
|--------------------------------------------------------------------------
| Test Identity Provider
|--------------------------------------------------------------------------
|
| Tests sign people in through an identity provider that runs in the test
| itself, never a real one. It signs answers with a key of its own and can
| break them the ways attackers do: no signature, a second unsigned
| assertion, a signature wrapped around other content, a comment inside a
| name. It refuses to exist outside a test binary, so it can never stand
| in for a real identity provider.
|
*/

package samltest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/dimmihq/dimmi/framework/saml"
)

/*
ErrNotTesting means the identity provider was asked for outside a test
binary.
*/
var ErrNotTesting = errors.New("samltest: only available in tests")

/*
Namespaces and values of the documents built here.
*/
const (
	protocolNS    = "urn:oasis:names:tc:SAML:2.0:protocol"
	assertionNS   = "urn:oasis:names:tc:SAML:2.0:assertion"
	statusOK      = "urn:oasis:names:tc:SAML:2.0:status:Success"
	subjectMethod = "urn:oasis:names:tc:SAML:2.0:cm:bearer"
	EmailFormat   = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	timeLayout    = "2006-01-02T15:04:05Z"
	redirectBind  = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
)

/*
IDP is an identity provider with its own signing key.
*/
type IDP struct {
	EntityID    string
	SSOURL      string
	key         *rsa.PrivateKey
	certificate *x509.Certificate
	pem         string
}

/*
New returns an identity provider with a fresh key, its certificate valid
from now.
*/
func New(entityID, ssoURL string, now time.Time) (*IDP, error) {
	if !testing.Testing() {
		return nil, ErrNotTesting
	}

	pair, err := saml.NewKeyPair(entityID, now)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode([]byte(pair.PrivateKey))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("read test key: %w", err)
	}

	certificate, err := saml.ParseCertificate(pair.Certificate)
	if err != nil {
		return nil, err
	}

	return &IDP{EntityID: entityID, SSOURL: ssoURL, key: key, certificate: certificate, pem: pair.Certificate}, nil
}

/*
Certificate returns the certificate the identity provider signs with.
*/
func (i *IDP) Certificate() *x509.Certificate {
	return i.certificate
}

/*
CertificatePEM returns that certificate PEM encoded.
*/
func (i *IDP) CertificatePEM() string {
	return i.pem
}

/*
Metadata returns the identity provider's metadata document.
*/
func (i *IDP) Metadata() []byte {
	doc := etree.NewDocument()
	entity := doc.CreateElement("md:EntityDescriptor")
	entity.CreateAttr("xmlns:md", "urn:oasis:names:tc:SAML:2.0:metadata")
	entity.CreateAttr("xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	entity.CreateAttr("entityID", i.EntityID)

	idp := entity.CreateElement("md:IDPSSODescriptor")
	idp.CreateAttr("protocolSupportEnumeration", protocolNS)

	key := idp.CreateElement("md:KeyDescriptor")
	key.CreateAttr("use", "signing")
	key.CreateElement("ds:KeyInfo").CreateElement("ds:X509Data").CreateElement("ds:X509Certificate").
		SetText(base64.StdEncoding.EncodeToString(i.certificate.Raw))

	sso := idp.CreateElement("md:SingleSignOnService")
	sso.CreateAttr("Binding", redirectBind)
	sso.CreateAttr("Location", i.SSOURL)

	out, _ := doc.WriteToBytes()

	return out
}

/*
signer returns a signing context on the identity provider's key, with
the exclusive canonicalization identity providers use, so a signed
assertion still verifies once placed in its response.
*/
func (i *IDP) signer() *dsig.SigningContext {
	store := dsig.TLSCertKeyStore(tls.Certificate{Certificate: [][]byte{i.certificate.Raw}, PrivateKey: i.key})

	ctx := dsig.NewDefaultSigningContext(store)
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

	return ctx
}

/*
newID returns a fresh XML id.
*/
func newID() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)

	return fmt.Sprintf("_%x", raw)
}
