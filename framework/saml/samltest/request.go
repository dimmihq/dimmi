/*
|--------------------------------------------------------------------------
| Test Requests
|--------------------------------------------------------------------------
|
| Reads back the request a service provider sent through a redirect, so a
| test can answer exactly that request, the way a real identity provider
| would.
|
*/

package samltest

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"regexp"
)

/*
requestID finds the id attribute of an authentication request.
*/
var requestID = regexp.MustCompile(`\sID="([^"]+)"`)

/*
ErrNoRequest means the address carries no readable SAML request.
*/
var ErrNoRequest = errors.New("samltest: no saml request in the address")

/*
Request reads the authentication request a service provider sends to the
identity provider through a redirect: the id the answer must name, and
the relay state the answer must carry back.
*/
func Request(address string) (id, relayState string, err error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return "", "", errors.Join(ErrNoRequest, err)
	}

	query := parsed.Query()

	deflated, err := base64.StdEncoding.DecodeString(query.Get("SAMLRequest"))
	if err != nil {
		return "", "", errors.Join(ErrNoRequest, err)
	}

	document, err := io.ReadAll(flate.NewReader(bytes.NewReader(deflated)))
	if err != nil {
		return "", "", errors.Join(ErrNoRequest, err)
	}

	match := requestID.FindSubmatch(document)
	if match == nil {
		return "", "", ErrNoRequest
	}

	return string(match[1]), query.Get("RelayState"), nil
}
