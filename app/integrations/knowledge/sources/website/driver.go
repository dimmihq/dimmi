/*
|--------------------------------------------------------------------------
| Website Driver
|--------------------------------------------------------------------------
|
| The website source reads a site through a crawler. The built-in one
| is a polite crawler dimmi runs itself; a crawl service, such as
| Cloudflare's, renders pages in a real browser so sites built with
| JavaScript read too, at the price of being slower. Either one lists a
| site and reads a page; checking a site's address is always a plain
| request, so adding a source never spends browser time.
|
| Pages a member ticked are listed with plain requests, since their
| addresses are already known, and only read through the crawl service.
| When the service refuses a crawl or a page, the built-in crawler does
| the work instead and the service's reason is logged, so a problem at
| the service never stops a sync.
|
*/

package website

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Crawler reads a site for the website source.
*/
type Crawler interface {
	/*
		List returns a page of the site's pages, from the cursor. A
		crawler that reads pages while listing hands each one back
		with the reference, so it is not fetched again.
	*/
	List(ctx context.Context, s Site, cursor string) (sources.Page, error)

	/*
		Fetch reads one page.
	*/
	Fetch(ctx context.Context, s Site, ref sources.Ref) (sources.Document, error)
}

/*
Driver is the website source, reading sites through its crawler.
*/
type Driver struct {
	builtin *Builtin
	crawler Crawler
}

/*
New returns the website driver crawling with the built-in crawler.
*/
func New(builtin *Builtin) *Driver {
	return &Driver{builtin: builtin, crawler: builtin}
}

/*
Through returns the driver crawling with another crawler.
*/
func (d *Driver) Through(c Crawler) *Driver {
	return &Driver{builtin: d.builtin, crawler: c}
}

/*
Describe says what the driver is.
*/
func (d *Driver) Describe() sources.Description {
	return sources.Description{
		Type: "website", Name: "Website", Logo: "website", Auth: sources.AuthNone,
		Summary: "Crawl your docs or site. Pages stay in sync and are cited by their address.",
		Schema:  json.RawMessage(schema), Modes: []sources.Mode{sources.ModeSync},
		Targets:   []sources.Target{sources.TargetAgent, sources.TargetOperator},
		Schedules: []string{"weekly", "daily", "manual"},
	}
}

/*
MaxDocuments is the page limit of the settings.
*/
func (d *Driver) MaxDocuments(cfg sources.Config) int {
	s, err := read(cfg)
	if err != nil {
		return defaultMaxPages
	}

	return s.MaxPages
}

/*
Check reads the start page with a plain request.
*/
func (d *Driver) Check(ctx context.Context, cfg sources.Config) (sources.CheckResult, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.CheckResult{}, err
	}

	return d.builtin.Check(ctx, s)
}

/*
List returns a page of the site's pages.
*/
func (d *Driver) List(ctx context.Context, cfg sources.Config, cursor string) (sources.Page, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Page{}, err
	}

	if len(s.Include) > 0 {
		return d.builtin.List(ctx, s, cursor)
	}

	page, err := d.crawler.List(ctx, s, cursor)
	if d.refused(ctx, err) {
		log.From(ctx).WarnContext(ctx, "crawl service refused a crawl, using the built-in crawler", log.Err(err))

		return d.builtin.List(ctx, s, "")
	}

	return page, err
}

/*
refused reports whether the crawl service failed in a way the built-in
crawler can make up for: any failure but being asked to wait, a page
that is gone or left out, and the caller giving up.
*/
func (d *Driver) refused(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil || d.crawler == Crawler(d.builtin) {
		return false
	}

	_, wait := errors.AsType[*sources.WaitError](err)
	_, limited := errors.AsType[*sources.RateLimitError](err)
	_, excluded := errors.AsType[*sources.ExcludedError](err)

	return !wait && !limited && !excluded && !errors.Is(err, sources.ErrNotFound)
}

/*
Fetch reads one page.
*/
func (d *Driver) Fetch(ctx context.Context, cfg sources.Config, ref sources.Ref) (sources.Document, error) {
	s, err := read(cfg)
	if err != nil {
		return sources.Document{}, err
	}

	doc, err := d.crawler.Fetch(ctx, s, ref)
	if d.refused(ctx, err) {
		log.From(ctx).WarnContext(ctx, "crawl service refused a page, using the built-in crawler",
			log.String("url", ref.URL), log.Err(err))

		return d.builtin.Fetch(ctx, s, ref)
	}

	return doc, err
}
