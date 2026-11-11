/*
|--------------------------------------------------------------------------
| Website Source
|--------------------------------------------------------------------------
|
| A customer's own docs or marketing site, read the way a polite crawler
| reads it. The sitemap lists the pages when there is one; otherwise the
| crawl starts at one address and follows links that stay on the site.
| robots.txt is always obeyed and every host is asked for at most one
| page a second, slower when the site sets a crawl delay. Each page
| keeps only its main content, turned into Markdown, and is keyed by
| its canonical address so the same page under two addresses is read
| once. A page that renders only with JavaScript is set aside with that
| reason rather than indexed empty.
|
*/

package website

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/robots"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Crawl limits: pages per source by default and at most, the largest
page read, how long a robots.txt is trusted, and the longest crawl
delay honored.
*/
const (
	defaultMaxPages = 500
	maxMaxPages     = 5000
	maxPageBytes    = 5 << 20
	robotsTTL       = time.Hour
	maxCrawlDelay   = 10 * time.Second
	pageTimeout     = 20 * time.Second
	burst           = 4
)

/*
UserAgent is how the crawler names itself to sites.
*/
const UserAgent = "DimmiBot/1.0 (+https://dimmi.ai/bot)"

/*
schema is the settings form of a website source.
*/
const schema = `{
  "type": "object",
  "required": ["start_url"],
  "properties": {
    "start_url": {"type": "string", "format": "uri", "title": "Website address",
      "description": "Where the crawl starts, such as https://docs.example.com.", "x-placeholder": "https://docs.example.com"},
    "sitemap_url": {"type": "string", "format": "uri", "title": "Sitemap",
      "description": "Defaults to /sitemap.xml when the site has one."},
    "follow_links": {"type": "boolean", "title": "Follow links", "default": true,
      "description": "Also read pages linked from the pages found."},
    "max_pages": {"type": "integer", "title": "Most pages", "default": 500, "minimum": 1, "maximum": 5000},
    "pages": {"type": "array", "items": {"type": "string"}, "format": "x-tree", "x-patterns": true,
      "title": "Pages", "description": "The parts of the site to read. The whole site when none are chosen."}
  }
}`

/*
Site is a website source's settings: where the crawl starts, the
sitemap, whether links are followed and the most pages read, with the
member's path patterns.
*/
type Site struct {
	StartURL    string   `json:"start_url"`
	SitemapURL  string   `json:"sitemap_url"`
	FollowLinks *bool    `json:"follow_links"`
	MaxPages    int      `json:"max_pages"`
	Include     []string `json:"-"`
	Exclude     []string `json:"-"`
}

/*
Builtin is the crawler dimmi runs itself. HTTP reaches sites, and
Clock and Interval pace them: the least time between two requests to
one host, none at zero.
*/
type Builtin struct {
	HTTP     *httpclient.Client
	Clock    clock.Clock
	Interval time.Duration

	mu      sync.Mutex
	robots  map[string]cachedRobots
	pacers  map[int64]*ratelimit.Local
	pacerMu sync.Mutex
}

/*
cachedRobots is one host's robots.txt and when it was read.
*/
type cachedRobots struct {
	rules *robots.Rules
	at    time.Time
}

/*
NewBuiltin returns the crawler dimmi runs itself.
*/
func NewBuiltin(client *httpclient.Client, clk clock.Clock, interval time.Duration) *Builtin {
	return &Builtin{
		HTTP: client, Clock: clk, Interval: interval, robots: map[string]cachedRobots{},
		pacers: map[int64]*ratelimit.Local{},
	}
}

/*
Check reads the start page.
*/
func (d *Builtin) Check(ctx context.Context, s Site) (sources.CheckResult, error) {
	resp, err := d.get(ctx, s.StartURL)
	if err != nil {
		return sources.CheckResult{}, startError("The site could not be reached.")
	}

	if err := sources.FromResponse(resp, "read start page"); err != nil {
		return sources.CheckResult{}, startError("The site answered with an error.")
	}

	start, _ := url.Parse(s.StartURL)
	urls, _ := d.sitemapURLs(ctx, s, start)

	return sources.CheckResult{Account: start.Host, Estimate: min(max(len(urls), 1), s.MaxPages)}, nil
}

/*
startError blames the start address.
*/
func startError(message string) error {
	return &sources.ConfigError{Field: "start_url", Message: message}
}

/*
read decodes and checks the settings.
*/
func read(cfg sources.Config) (Site, error) {
	var s Site
	if err := cfg.Decode(&s); err != nil {
		return s, err
	}

	s.Include, s.Exclude = cfg.Include, cfg.Exclude

	start, err := url.Parse(strings.TrimSpace(s.StartURL))
	if err != nil || (start.Scheme != "https" && start.Scheme != "http") || start.Host == "" {
		return s, &sources.ConfigError{Field: "start_url", Message: "Enter the full address, starting with https://."}
	}

	s.StartURL = Canonical(start)
	if s.MaxPages <= 0 {
		s.MaxPages = defaultMaxPages
	}

	s.MaxPages = min(s.MaxPages, maxMaxPages)

	return s, nil
}

/*
Follows reports whether the crawl follows links, the default.
*/
func (s Site) Follows() bool {
	return s.FollowLinks == nil || *s.FollowLinks
}

/*
get fetches an address as the crawler, paced per host and bounded in
size and time.
*/
func (d *Builtin) get(ctx context.Context, address string) (*httpclient.Response, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if err := d.pace(ctx, u.Host); err != nil {
		return nil, err
	}

	return d.HTTP.New().WithHeader("User-Agent", UserAgent).
		Accept("text/html, text/markdown, text/plain;q=0.9, application/xml;q=0.8").
		MaxBody(maxPageBytes).Timeout(pageTimeout).Get(ctx, address)
}

/*
pace takes the host's turn: one request a second with short bursts, or
the crawl delay the host's robots.txt asks for, capped at ten seconds.
When it is not the host's turn yet the request is not sent; the caller
is asked to come back, so a worker never sits waiting on a slow site.
*/
func (d *Builtin) pace(_ context.Context, host string) error {
	if d.Interval <= 0 {
		return nil
	}

	interval := d.Interval

	d.mu.Lock()
	cached, ok := d.robots[host]
	d.mu.Unlock()

	if ok && cached.rules != nil {
		interval = min(max(interval, cached.rules.CrawlDelay(UserAgent)), maxCrawlDelay)
	}

	d.pacerMu.Lock()
	pacer, ok := d.pacers[interval.Milliseconds()]

	if !ok {
		var err error

		pacer, err = ratelimit.NewLocal(interval, burst, d.Clock)
		if err != nil {
			d.pacerMu.Unlock()

			return err
		}

		d.pacers[interval.Milliseconds()] = pacer
	}
	d.pacerMu.Unlock()

	if !pacer.Allow(host) {
		return &sources.RateLimitError{RetryAfter: interval}
	}

	return nil
}

/*
rules returns the host's robots.txt, read at most once an hour.
*/
func (d *Builtin) rules(ctx context.Context, u *url.URL) (*robots.Rules, error) {
	d.mu.Lock()
	cached, ok := d.robots[u.Host]
	d.mu.Unlock()

	if ok && d.Clock.Now().Sub(cached.at) < robotsTTL {
		return cached.rules, nil
	}

	rules, err := robots.Fetch(ctx, d.HTTP, u.Scheme+"://"+u.Host)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	d.robots[u.Host] = cachedRobots{rules: rules, at: d.Clock.Now()}
	d.mu.Unlock()

	return rules, nil
}

/*
allowed reports whether robots.txt lets the crawler read the address.
*/
func (d *Builtin) allowed(ctx context.Context, u *url.URL) (bool, error) {
	rules, err := d.rules(ctx, u)
	if err != nil {
		return false, err
	}

	target := u.EscapedPath()
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}

	return rules.Allowed(UserAgent, target), nil
}
