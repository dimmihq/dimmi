/*
|--------------------------------------------------------------------------
| Identity Provider Metadata
|--------------------------------------------------------------------------
|
| An administrator hands over their identity provider's metadata instead
| of copying its parts one by one. Only what a sign-in needs is read: the
| entity id, where to send a sign-in by redirect, and the certificates it
| signs with, all of them, so a certificate rollover keeps working. A
| provider that cannot take a sign-in by redirect cannot be used.
|
*/

package saml

import (
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
)

/*
ErrInvalidMetadata means the document is not identity provider
metadata this package can read. ErrNoRedirectBinding means the provider
takes no sign-in by redirect.
*/
var (
	ErrInvalidMetadata   = errors.New("saml: invalid identity provider metadata")
	ErrNoRedirectBinding = errors.New("saml: identity provider has no redirect binding")
)

/*
maxMetadata is the largest metadata document read.
*/
const maxMetadata = 1 << 20

/*
ParseMetadata reads an identity provider's metadata, alone or the first
identity provider of a list.
*/
func ParseMetadata(document []byte) (IdentityProvider, error) {
	if len(document) == 0 || len(document) > maxMetadata {
		return IdentityProvider{}, ErrInvalidMetadata
	}

	descriptor, err := entity(document)
	if err != nil {
		return IdentityProvider{}, err
	}

	idp := descriptor.IDPSSODescriptor
	out := IdentityProvider{EntityID: descriptor.EntityID}

	for _, service := range idp.SingleSignOnServices {
		if service.Binding == saml2.BindingHttpRedirect {
			out.SSOURL = service.Location

			break
		}
	}

	if out.SSOURL == "" {
		return IdentityProvider{}, ErrNoRedirectBinding
	}

	if out.Certificates, err = signingCertificates(idp.KeyDescriptors); err != nil {
		return IdentityProvider{}, err
	}

	if out.EntityID == "" || len(out.Certificates) == 0 {
		return IdentityProvider{}, ErrInvalidMetadata
	}

	return out, nil
}

/*
entity returns the identity provider's entity descriptor, unwrapping a
list of entities.
*/
func entity(document []byte) (types.EntityDescriptor, error) {
	var single types.EntityDescriptor
	if err := xml.Unmarshal(document, &single); err == nil && single.IDPSSODescriptor != nil {
		return single, nil
	}

	var list struct {
		Entities []types.EntityDescriptor `xml:"EntityDescriptor"`
	}

	if err := xml.Unmarshal(document, &list); err != nil {
		return types.EntityDescriptor{}, ErrInvalidMetadata
	}

	for _, e := range list.Entities {
		if e.IDPSSODescriptor != nil {
			return e, nil
		}
	}

	return types.EntityDescriptor{}, ErrInvalidMetadata
}

/*
signingCertificates returns the certificates the identity provider
signs with: those marked for signing or for no use in particular.
*/
func signingCertificates(keys []types.KeyDescriptor) ([]*x509.Certificate, error) {
	var out []*x509.Certificate

	for _, key := range keys {
		if key.Use != "" && key.Use != "signing" {
			continue
		}

		for _, data := range key.KeyInfo.X509Data.X509Certificates {
			certificate, err := ParseCertificate(strings.TrimSpace(data.Data))
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrInvalidMetadata, err)
			}

			out = append(out, certificate)
		}
	}

	return out, nil
}
