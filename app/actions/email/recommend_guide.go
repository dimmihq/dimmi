/*
|--------------------------------------------------------------------------
| Recommend a Setup Guide
|--------------------------------------------------------------------------
|
| An address is set up in the mail service it lives in, and the page
| opens on that service's steps. A public domain such as gmail.com names
| its service; any other domain is read from its mail exchangers. The
| answer is remembered for an hour per domain, and a lookup that fails or
| takes over three seconds falls back to the generic steps rather than
| holding up the page.
|
*/

package email

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/dns"

	"github.com/dimmihq/dimmi/app/integrations/email/setupguides"
)

/*
How long a domain's mail service is remembered, and how long its lookup
may take.
*/
const (
	mailHostTTL     = time.Hour
	mailHostTimeout = 3 * time.Second
)

/*
RecommendGuide picks the setup guide that fits an address. DNS reads the
domain's mail exchangers and Cache remembers the answer.
*/
type RecommendGuide struct {
	DNS   dns.Resolver
	Cache *cache.Store
}

/*
Handle returns the key of the guide for the address.
*/
func (r *RecommendGuide) Handle(ctx context.Context, address string) string {
	domain := strings.ToLower(address[strings.LastIndex(address, "@")+1:])
	if guide, ok := setupguides.ForDomain(domain); ok {
		return guide
	}

	key := "email:mail_host:" + domain
	if cached, err := r.Cache.Get(ctx, key); err == nil && len(cached) > 0 {
		return string(cached)
	}

	guide, ok := r.lookup(ctx, domain)
	if ok {
		_ = r.Cache.Set(ctx, key, []byte(guide), mailHostTTL)
	}

	return guide
}

/*
lookup reads the domain's mail exchangers. A domain without any is
answered for good; a failed lookup reports false, so it is not remembered
and the next page load tries again.
*/
func (r *RecommendGuide) lookup(ctx context.Context, domain string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, mailHostTimeout)
	defer cancel()

	exchangers, err := r.DNS.MX(ctx, domain)
	if errors.Is(err, dns.ErrNotFound) {
		return setupguides.Generic, true
	}

	if err != nil {
		return setupguides.Generic, false
	}

	hosts := make([]string, 0, len(exchangers))
	for _, mx := range exchangers {
		hosts = append(hosts, mx.Host)
	}

	return setupguides.ForExchangers(hosts), true
}
