/*
|--------------------------------------------------------------------------
| Endpoint URLs
|--------------------------------------------------------------------------
|
| An endpoint receives the workspace's data, so its URL is checked
| before anything is sent: a public http or https address, https where
| the install requires it, no credentials in it, an ordinary port, and
| never a private network, this server's own inbound webhooks or a
| loopback name. The host is resolved once here; every request checks
| again where it connects, since a name can be pointed elsewhere later.
| A fragment is never sent, so it is dropped, and an international host
| is stored in its ASCII form.
|
*/

package webhooks

import (
	"context"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
The longest endpoint URL, and the lowest port above the well-known ones.
*/
const (
	maxURLLength   = 2048
	lowestOpenPort = 1024
	httpPort       = "80"
	httpsPort      = "443"
	inboundPrefix  = "/webhooks/"
)

/*
NormalizeURL checks an endpoint URL and returns the form it is stored
in.
*/
func (w *Webhooks) NormalizeURL(ctx context.Context, raw string) (string, error) {
	u, err := parseURL(raw)
	if err != nil {
		return "", err
	}

	if u.Scheme == "http" && w.Config.HTTPSRequired(w.Edition) {
		return "", exceptions.ErrInsecureWebhookURL
	}

	host, err := httpclient.ASCIIHost(u.Hostname())
	if err != nil {
		return "", exceptions.ErrInvalidWebhookURL
	}

	port := u.Port()
	if !allowedPort(port) {
		return "", exceptions.ErrInvalidWebhookURL
	}

	u.Host = joinHost(host, port)

	if w.isOwnInbound(u) {
		return "", exceptions.ErrInvalidWebhookURL
	}

	if err := w.checkDestination(ctx, host, defaultPort(u.Scheme, port)); err != nil {
		return "", err
	}

	return u.String(), nil
}

/*
parseURL reads an absolute http or https URL without credentials and
drops its fragment.
*/
func parseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLLength {
		return nil, exceptions.ErrInvalidWebhookURL
	}

	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return nil, exceptions.ErrInvalidWebhookURL
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, exceptions.ErrInvalidWebhookURL
	}

	u.Fragment, u.RawFragment = "", ""

	return u, nil
}

/*
allowedPort accepts the default port, 80, 443, and any port above the
well-known ones; the rest, such as 22 or 25, belong to other services.
*/
func allowedPort(port string) bool {
	if port == "" || port == httpPort || port == httpsPort {
		return true
	}

	n, err := strconv.Atoi(port)

	return err == nil && n >= lowestOpenPort && n <= 65535
}

/*
joinHost writes a host with its port, bracketing an IPv6 literal.
*/
func joinHost(host, port string) string {
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	if port == "" {
		return host
	}

	return host + ":" + port
}

/*
defaultPort returns the port a request to the URL connects to.
*/
func defaultPort(scheme, port string) string {
	if port != "" {
		return port
	}

	if scheme == "http" {
		return httpPort
	}

	return httpsPort
}

/*
isOwnInbound reports a URL at this server's own inbound webhooks, which
would deliver events back into the product.
*/
func (w *Webhooks) isOwnInbound(u *url.URL) bool {
	own, err := url.Parse(w.AppURL)
	if err != nil || own.Host == "" {
		return false
	}

	return strings.EqualFold(own.Host, u.Host) && strings.HasPrefix(u.Path, inboundPrefix)
}

/*
checkDestination refuses a host the client may not reach: a private or
loopback literal, localhost, or a name resolving to such an address. A
name that does not resolve yet is accepted; its requests fail and are
retried until it does.
*/
func (w *Webhooks) checkDestination(ctx context.Context, host, port string) error {
	if w.HTTP.CheckAddress(joinHost(host, port)) != nil {
		return exceptions.ErrInvalidWebhookURL
	}

	if _, err := netip.ParseAddr(host); err == nil || w.DNS == nil {
		return nil
	}

	addrs, _ := w.DNS.Addrs(ctx, host)
	for _, addr := range addrs {
		if w.HTTP.CheckAddress(joinHost(addr.String(), port)) != nil {
			return exceptions.ErrInvalidWebhookURL
		}
	}

	return nil
}
