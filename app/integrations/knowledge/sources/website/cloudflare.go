/*
|--------------------------------------------------------------------------
| Cloudflare Crawler
|--------------------------------------------------------------------------
|
| Cloudflare's Browser Run renders every page in a real browser, so a
| site built with JavaScript reads as well as a server-rendered one. A
| listing starts one crawl job for the whole site, which Cloudflare runs
| on its own, obeying robots.txt; the listing comes back every little
| while until the job ends, then pages through its results, each page
| already in Markdown, so nothing is fetched twice. Reading one page on
| its own, to sync a single document again, asks for that page's
| Markdown. It is slower than the built-in crawler but sees what a
| visitor sees.
|
*/

package website

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Cloudflare API locations, how long to wait between looks at a running
job, and the longest a call to it may take.
*/
const (
	cloudflareAPI     = "https://api.cloudflare.com/client/v4"
	cloudflarePoll    = 15 * time.Second
	cloudflareTimeout = 90 * time.Second
	cloudflareMaxWait = 15 * time.Minute
	waitCursor        = "wait:"
	pageCursor        = "page:"
)

/*
ErrCloudflareAccount means the token reaches no Cloudflare account.
*/
var ErrCloudflareAccount = errors.New("website: the Cloudflare token reaches no account")

/*
Cloudflare crawls through Cloudflare's Browser Run. Token is an API
token with Browser Run edit access; AccountID is found from the token
when left empty. Clock times the wait for a crawl: one still running
after MaxWait, fifteen minutes when zero, is stopped and what it read
so far is kept, so a page that never finishes loading cannot hold the
sync forever.
*/
type Cloudflare struct {
	HTTP      *httpclient.Client
	Clock     clock.Clock
	Token     string
	AccountID string
	MaxWait   time.Duration

	mu sync.Mutex
}

/*
envelope is every Cloudflare API answer.
*/
type envelope[T any] struct {
	Success bool `json:"success"`
	Result  T    `json:"result"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

/*
crawlJob is a crawl job and a page of its results.
*/
type crawlJob struct {
	ID      string        `json:"id"`
	Status  string        `json:"status"`
	Total   int           `json:"total"`
	Records []crawlRecord `json:"records"`
	Cursor  any           `json:"cursor"`
}

/*
browserSeconds reads how long Cloudflare's browser ran for a job, a
field it names in its own camel case.
*/
func browserSeconds(raw json.RawMessage) float64 {
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	seconds, _ := fields["browserSecondsUsed"].(float64)

	return seconds
}

/*
BrowserService names Cloudflare's hosted browser in the usage it
reports, once per crawl.
*/
const BrowserService = "cloudflare:browser-run"

/*
crawlRecord is one crawled page.
*/
type crawlRecord struct {
	URL      string `json:"url"`
	Outcome  string `json:"outcome"`
	Markdown string `json:"markdown"`
	Metadata struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"metadata"`
}

/*
List starts the site's crawl job, waits for it, then pages through
what it read. The cursor says which: "wait:<job>:<started>" while the
job runs, "page:<job>:<cursor>" while its results are read.
*/
func (c *Cloudflare) List(ctx context.Context, s Site, cursor string) (sources.Page, error) {
	switch {
	case cursor == "":
		id, err := c.start(ctx, s)
		if err != nil {
			return sources.Page{}, err
		}

		return sources.Page{Next: waitCursor + id + ":" + strconv.FormatInt(c.Clock.Now().Unix(), 10)}, nil
	case strings.HasPrefix(cursor, waitCursor):
		id, started, _ := strings.Cut(strings.TrimPrefix(cursor, waitCursor), ":")
		if err := c.finished(ctx, id, started); err != nil {
			return sources.Page{}, err
		}

		return c.results(ctx, s, id, "")
	}

	id, after, _ := strings.Cut(strings.TrimPrefix(cursor, pageCursor), ":")

	return c.results(ctx, s, id, after)
}

/*
start asks Cloudflare to crawl the site and returns the job's id.
*/
func (c *Cloudflare) start(ctx context.Context, s Site) (string, error) {
	source := "all"
	if !s.Follows() {
		source = "sitemaps"
	}

	body := map[string]any{
		"url": s.StartURL, "limit": s.MaxPages, "source": source, "formats": []string{"markdown"},
		"crawlPurposes": []string{"search", "ai-input"}, "contentUse": "reference",
		"options": map[string]any{
			"includePatterns": onSite(s.StartURL, s.Include), "excludePatterns": onSite(s.StartURL, s.Exclude),
		},
	}

	var answer envelope[string]
	if err := c.call(ctx, cfRequest{method: "POST", path: "/crawl", body: body, out: &answer}); err != nil {
		return "", err
	}

	return answer.Result, nil
}

/*
onSite turns path patterns into the address patterns Cloudflare reads:
"/docs/**" on https://acme.com reads "https://acme.com/docs/**".
*/
func onSite(start string, patterns []string) []string {
	u, err := url.Parse(start)
	if err != nil {
		return []string{}
	}

	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, u.Scheme+"://"+u.Host+"/"+strings.TrimPrefix(p, "/"))
	}

	return out
}

/*
finished waits for the job: a WaitError while it runs, nil once it is
done or was stopped for running too long, an error when it failed.
*/
func (c *Cloudflare) finished(ctx context.Context, id, started string) error {
	var answer envelope[crawlJob]
	poll := cfRequest{method: "GET", path: "/crawl/" + url.PathEscape(id) + "?limit=1", out: &answer}
	if err := c.call(ctx, poll); err != nil {
		return err
	}

	status := answer.Result.Status
	if status == "errored" {
		return fmt.Errorf("cloudflare crawl %s failed: %w", id, sources.ErrUnavailable)
	}

	if status != "running" && status != "queued" && status != "" {
		return nil
	}

	since, _ := strconv.ParseInt(started, 10, 64)
	if c.Clock.Now().Sub(time.Unix(since, 0)) < c.maxWait() {
		return &sources.WaitError{After: cloudflarePoll}
	}

	var stopped envelope[any]

	return c.call(ctx, cfRequest{method: "DELETE", path: "/crawl/" + url.PathEscape(id), out: &stopped})
}

func (c *Cloudflare) maxWait() time.Duration {
	if c.MaxWait > 0 {
		return c.MaxWait
	}

	return cloudflareMaxWait
}

/*
results reads one page of the finished job's pages, each with its
Markdown.
*/
func (c *Cloudflare) results(ctx context.Context, s Site, id, after string) (sources.Page, error) {
	path := "/crawl/" + url.PathEscape(id) + "?status=completed"
	if after != "" {
		path += "&cursor=" + url.QueryEscape(after)
	}

	var raw envelope[json.RawMessage]
	if err := c.call(ctx, cfRequest{method: "GET", path: path, out: &raw}); err != nil {
		return sources.Page{}, err
	}

	var answer envelope[crawlJob]
	if err := json.Unmarshal(raw.Result, &answer.Result); err != nil {
		return sources.Page{}, fmt.Errorf("read cloudflare crawl %s: %w", id, err)
	}

	start, _ := url.Parse(s.StartURL)
	out := sources.Page{}

	if after == "" {
		out.Usage = sources.Usage{Service: BrowserService, BrowserMs: int64(browserSeconds(raw.Result) * 1000)}
	}

	for _, r := range answer.Result.Records {
		if ref, ok := recordRef(start, r); ok {
			out.Refs = append(out.Refs, ref)
		}
	}

	if next := cursorOf(answer.Result.Cursor); next != "" {
		out.Next = pageCursor + id + ":" + next
	}

	return out, nil
}

/*
recordRef turns a crawled page of the site into a reference carrying
its document; pages of other sites and empty pages are left out.
*/
func recordRef(start *url.URL, r crawlRecord) (sources.Ref, bool) {
	address := r.Metadata.URL
	if address == "" {
		address = r.URL
	}

	u, err := url.Parse(address)
	if err != nil || !sameSite(start, u) || strings.TrimSpace(r.Markdown) == "" {
		return sources.Ref{}, false
	}

	canonical := Canonical(u)
	md := sources.Tidy(r.Markdown)
	title := withoutSite(r.Metadata.Title)

	if title == "" {
		title = sources.Humanize(u.Path)
	}

	doc := &sources.Document{
		Title: title, Markdown: md, URL: canonical, Path: u.EscapedPath(), Visibility: sources.VisibilityPublic,
	}

	return sources.Ref{
		ExternalID: canonical, URL: canonical, Path: u.EscapedPath(), Title: title,
		Version: hashing.SHA256Hex([]byte(title + "\n" + md)), Document: doc,
	}, true
}

/*
cursorOf reads Cloudflare's cursor, a number or a string, empty when
there is none.
*/
func cursorOf(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case float64:
		return fmt.Sprintf("%.0f", c)
	}

	return ""
}

/*
Fetch reads one page as Markdown, rendered in a browser.
*/
func (c *Cloudflare) Fetch(ctx context.Context, _ Site, ref sources.Ref) (sources.Document, error) {
	u, err := url.Parse(ref.URL)
	if err != nil {
		return sources.Document{}, sources.ErrNotFound
	}

	var answer envelope[string]
	read := cfRequest{method: "POST", path: "/markdown", body: map[string]any{"url": ref.URL}, out: &answer}
	if err := c.call(ctx, read); err != nil {
		return sources.Document{}, err
	}

	md := sources.Tidy(answer.Result)
	if md == "" {
		return sources.Document{}, sources.Excluded("empty")
	}

	doc := sources.MarkdownFile(u.Path, []byte(md))
	if strings.HasPrefix(doc.Title, sources.Humanize(u.Path)) && ref.Title != "" {
		doc.Title = ref.Title
	}

	doc.URL, doc.Path, doc.Visibility = Canonical(u), u.EscapedPath(), sources.VisibilityPublic

	return doc, nil
}

/*
cfRequest is one call to Browser Run: its method, path, body and
where the answer is decoded.
*/
type cfRequest struct {
	method string
	path   string
	body   any
	out    any
}

/*
call sends one request to the account's Browser Run API and decodes
the answer.
*/
func (c *Cloudflare) call(ctx context.Context, r cfRequest) error {
	method, path, body, out := r.method, r.path, r.body, r.out

	account, err := c.account(ctx)
	if err != nil {
		return err
	}

	req := c.HTTP.New().WithToken(c.Token).Accept("application/json").Timeout(cloudflareTimeout)
	address := cloudflareAPI + "/accounts/" + account + "/browser-run" + path

	var resp *httpclient.Response

	switch method {
	case "POST":
		resp, err = req.WithHeader("Content-Type", "application/json").Post(ctx, address, body)
	case "DELETE":
		resp, err = req.Delete(ctx, address)
	default:
		resp, err = req.Get(ctx, address)
	}

	if err != nil {
		return fmt.Errorf("cloudflare %s: %w", path, err)
	}

	if err := sources.FromResponse(resp, "cloudflare "+path); err != nil {
		return err
	}

	return resp.JSON(out)
}

/*
account returns the account the token reaches, found once.
*/
func (c *Cloudflare) account(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.AccountID != "" {
		return c.AccountID, nil
	}

	resp, err := c.HTTP.New().WithToken(c.Token).Accept("application/json").Get(ctx, cloudflareAPI+"/accounts")
	if err != nil {
		return "", fmt.Errorf("cloudflare accounts: %w", err)
	}

	if err := sources.FromResponse(resp, "cloudflare accounts"); err != nil {
		return "", err
	}

	var answer envelope[[]struct {
		ID string `json:"id"`
	}]
	if err := resp.JSON(&answer); err != nil || len(answer.Result) == 0 {
		return "", ErrCloudflareAccount
	}

	c.AccountID = answer.Result[0].ID

	return c.AccountID, nil
}
