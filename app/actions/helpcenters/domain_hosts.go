/*
|--------------------------------------------------------------------------
| Help Centers On Custom Domains
|--------------------------------------------------------------------------
|
| A request on a workspace's own domain names its help center only by
| its host. Hosts are looked up through a security definer function and
| remembered for half a minute, the answer and the lack of one alike, so
| a busy help center costs one lookup every so often rather than one a
| request. A domain removed or moved stops answering within that time.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/memo"

	"github.com/dimmihq/dimmi/app/models"
)

/*
DomainHosts finds which help center answers on a custom domain.
*/
type DomainHosts struct {
	DB    *database.DB
	known *memo.Cache[string, string]
}

/*
NewDomainHosts returns a lookup remembering up to ten thousand hosts for
thirty seconds.
*/
func NewDomainHosts(db *database.DB) *DomainHosts {
	return &DomainHosts{DB: db, known: memo.New[string, string](memo.Options{Size: 10000, TTL: 30 * time.Second})}
}

/*
Subdomain returns the subdomain of the help center answering on the
host, empty when none does.
*/
func (d *DomainHosts) Subdomain(ctx context.Context, host string) string {
	host = strings.ToLower(host)
	if i := strings.LastIndexByte(host, ':'); i > 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	if host == "" || !strings.Contains(host, ".") {
		return ""
	}
	if sub, ok := d.known.Get(host); ok {
		return sub
	}
	row, err := models.New(d.DB).ResolveHelpCenterDomain(ctx, host)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return ""
	}
	d.known.Set(host, row.Subdomain)
	return row.Subdomain
}
