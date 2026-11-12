/*
|--------------------------------------------------------------------------
| Help Center Addresses
|--------------------------------------------------------------------------
|
| A help center answers at its subdomain of the install's help domain,
| or, when the install has none, under the app's own address. Pages link
| to each other relative to that root, so one rendering serves both.
|
*/

package helpcenters

import (
	"strings"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Addresses builds help centers' public addresses: BaseDomain is the help
domain, empty to serve under AppURL at /hc/<subdomain>.
*/
type Addresses struct {
	BaseDomain string
	AppURL     string
}

/*
Root is where the help center's pages start, without a trailing slash:
its verified custom domain when it has one.
*/
func (a Addresses) Root(h models.HelpCenter) string {
	if h.CustomDomain != nil && *h.CustomDomain != "" {
		return "https://" + *h.CustomDomain
	}
	if a.BaseDomain != "" {
		return "https://" + h.Subdomain + "." + a.BaseDomain
	}
	return strings.TrimSuffix(a.AppURL, "/") + "/hc/" + h.Subdomain
}

/*
Subdomain reads which help center a request's host names, empty when the
host is not under the help domain.
*/
func (a Addresses) Subdomain(host string) string {
	if a.BaseDomain == "" {
		return ""
	}
	host = strings.ToLower(host)
	if i := strings.LastIndexByte(host, ':'); i > 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	sub, ok := strings.CutSuffix(host, "."+a.BaseDomain)
	if !ok || sub == "" || strings.Contains(sub, ".") {
		return ""
	}
	return sub
}
