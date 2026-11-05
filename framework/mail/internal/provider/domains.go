/*
|--------------------------------------------------------------------------
| Provider Domain API Calls
|--------------------------------------------------------------------------
|
| Domain APIs fail like the send APIs, with two answers a caller must be
| able to tell apart: the domain is gone at the provider, which the caller
| shows as a broken setup, and the credential was refused, which only the
| operator can fix.
|
*/

package provider

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
CheckDomain returns nil for a 2xx response and the mail error kind for
anything else. The action names the call, such as "get domain".
*/
func CheckDomain(name, action string, resp *httpclient.Response, err error) error {
	if err == nil && resp.Status == http.StatusNotFound {
		return DomainNotFound(name, action)
	}

	label := name + " " + action
	if err == nil && (resp.Status == http.StatusUnauthorized || resp.Status == http.StatusForbidden) {
		return fmt.Errorf("%s: status %d: %w: %w", label, resp.Status, mail.ErrDomainAccessDenied, mail.ErrTemporary)
	}

	return classify(label, resp, err)
}

/*
DomainNotFound is the error for a domain the provider does not have, for
providers that say so in their own way and for an id that cannot exist.
*/
func DomainNotFound(name, action string) error {
	return fmt.Errorf("%s %s: %w: %w", name, action, mail.ErrDomainNotFound, mail.ErrPermanent)
}

/*
TrackingLeftOn is the error for a domain created with its tracking still
on.
*/
func TrackingLeftOn(name string, err error) error {
	return fmt.Errorf("%s create domain: %w: %w", name, mail.ErrDomainTrackingLeftOn, err)
}

/*
DecodeDomain reads a successful answer into the value. An answer that
cannot be read is temporary: the provider's API is misbehaving, and the
request itself was fine.
*/
func DecodeDomain(name, action string, resp *httpclient.Response, into any) error {
	if err := resp.JSON(into); err != nil {
		return fmt.Errorf("%s %s: %w: %w", name, action, mail.ErrTemporary, err)
	}

	return nil
}

/*
FullHost returns a record's host fully qualified under the domain, for
providers that answer with a name relative to it.
*/
func FullHost(host, domain string) string {
	host = strings.TrimSuffix(host, ".")

	switch {
	case host == "", host == "@":
		return domain
	case host == domain, strings.HasSuffix(host, "."+domain):
		return host
	default:
		return host + "." + domain
	}
}
