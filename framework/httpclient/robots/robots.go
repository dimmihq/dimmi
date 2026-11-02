/*
|--------------------------------------------------------------------------
| Robots Rules
|--------------------------------------------------------------------------
|
| A crawler fetches a site only where the site's robots.txt lets it, as
| RFC 9309 describes. When the file cannot be had the protocol decides:
| a missing file (any 4xx) allows everything, while a server error, an
| unreachable or refused host, or a file that cannot be parsed allows
| nothing, so a failure never turns into crawling against the site's
| wishes. Only the first 500 KiB of a file is read, the least the RFC
| requires crawlers to parse.
|
*/

package robots

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/temoto/robotstxt"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
ErrInvalidOrigin rejects an origin that is not an http or https URL
with a host.
*/
var ErrInvalidOrigin = errors.New("robots: origin must be an http or https url")

/*
MaxSize is how many bytes of a robots.txt are read; the rest is ignored.
*/
const MaxSize = 500 << 10

/*
Rules are one site's crawl rules.
*/
type Rules struct {
	data *robotstxt.RobotsData
}

/*
Fetch reads the robots.txt of the origin, such as https://example.com,
through the client. Only a malformed origin or the context ending is an
error; every other failure yields rules that allow nothing.
*/
func Fetch(ctx context.Context, client *httpclient.Client, origin string) (*Rules, error) {
	site, err := url.Parse(origin)
	if err != nil || (site.Scheme != "http" && site.Scheme != "https") || site.Host == "" {
		return nil, fmt.Errorf("%q: %w", origin, ErrInvalidOrigin)
	}

	resp, err := client.New().Stream().Get(ctx, site.Scheme+"://"+site.Host+"/robots.txt")
	if ctx.Err() != nil {
		return nil, fmt.Errorf("fetch robots.txt of %s: %w", site.Host, ctx.Err())
	}

	if err != nil {
		return disallowAll(), nil
	}

	defer func() { _ = resp.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Reader(), MaxSize))
	if err != nil {
		return disallowAll(), nil
	}

	data, err := robotstxt.FromStatusAndBytes(resp.Status, body)
	if err != nil {
		return disallowAll(), nil
	}

	return &Rules{data: data}, nil
}

/*
Allowed reports whether the crawler may fetch the path, which includes
any query string.
*/
func (r *Rules) Allowed(userAgent, path string) bool {
	return r.data.TestAgent(path, userAgent)
}

/*
CrawlDelay returns how long the crawler should wait between requests to
the site; zero when the site sets no delay.
*/
func (r *Rules) CrawlDelay(userAgent string) time.Duration {
	return r.data.FindGroup(userAgent).CrawlDelay
}

func disallowAll() *Rules {
	data, _ := robotstxt.FromStatusAndBytes(http.StatusInternalServerError, nil)

	return &Rules{data: data}
}
