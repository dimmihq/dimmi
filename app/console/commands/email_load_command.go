//go:build dev

/*
|--------------------------------------------------------------------------
| Email Load Run
|--------------------------------------------------------------------------
|
| Measures whether one replica keeps up with mail: it posts emails to a
| running install's test source at a steady rate, then reads back from the
| database how long each took from arrival to processed. The bar is 50 a
| second for ten minutes with the 95th percentile under ten seconds. Only
| development builds carry it, and it only talks to the address given.
|
*/

package commands

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
settleWait is how long the run waits for the last emails to be processed.
*/
const settleWait = 5 * time.Minute

/*
EmailLoadCommand posts emails at a rate and reports how fast they were
processed.
*/
type EmailLoadCommand struct {
	App *app.App
}

/*
Name returns email:load.
*/
func (c EmailLoadCommand) Name() string {
	return "email:load"
}

/*
Description says what the command does.
*/
func (c EmailLoadCommand) Description() string {
	return "Post emails to a running install's test source and measure processing time"
}

/*
Flags declares the install, the address, the rate and how long to run.
*/
func (c EmailLoadCommand) Flags(flags *console.Flags) {
	flags.String("url", "http://127.0.0.1:8080", "the running install")
	flags.String("to", "", "a forwarding address of the install")
	flags.String("secret", "", "the test source's secret")
	flags.Int("rate", 50, "emails a second")
	flags.String("duration", "1m", "how long to send, such as 10m")
}

/*
loadRun is one run's settings and what the sender saw.
*/
type loadRun struct {
	base, to, secret, prefix string
	rate                     int
	duration                 time.Duration

	mu       sync.Mutex
	accepted []time.Duration
	failed   int
}

/*
Handle sends, waits for processing to settle, and prints the figures.
*/
func (c EmailLoadCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	run, err := c.parse(in)
	if err != nil {
		return err
	}

	route, err := models.New(c.App.DB).RouteEmailAddress(ctx, models.RouteEmailAddressParams{
		DeletedSince: c.App.Clock.Now(), ForwardingAddress: run.to,
	})
	if err != nil {
		return fmt.Errorf("find %s: %w", run.to, err)
	}

	client, err := loadClient(run.base)
	if err != nil {
		return err
	}

	sent := c.send(ctx, client, run)
	out.Line("sent %d in %s, %d refused", sent, run.duration, run.failed)
	out.Line("webhook answer: p50 %s, p95 %s", percentile(run.accepted, 0.5), percentile(run.accepted, 0.95))

	return c.report(tenancy.With(ctx, route.WorkspaceID.String()), route, run, out)
}

func (c EmailLoadCommand) parse(in *console.Input) (*loadRun, error) {
	duration, err := time.ParseDuration(in.String("duration"))
	if err != nil || in.Int("rate") <= 0 || in.String("to") == "" {
		return nil, fmt.Errorf("--to, a positive --rate and a --duration such as 10m are required")
	}

	return &loadRun{
		base: in.String("url"), to: in.String("to"), secret: in.String("secret"), rate: in.Int("rate"),
		duration: duration, prefix: "load " + strconv.FormatInt(c.App.Clock.Now().Unix(), 10) + " ",
	}, nil
}

/*
loadClient allows exactly the install's own address, which is usually on
this machine.
*/
func loadClient(base string) (*httpclient.Client, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("--url: %w", err)
	}

	host := parsed.Host
	if parsed.Port() == "" {
		host = parsed.Host + ":80"
	}

	return httpclient.New(httpclient.Config{AllowHosts: []string{host}, Timeout: 30 * time.Second})
}

/*
send posts one email per tick until the duration ran out, then waits for
the posts in flight.
*/
func (c EmailLoadCommand) send(ctx context.Context, client *httpclient.Client, run *loadRun) int {
	ticker := c.App.Clock.NewTicker(time.Second / time.Duration(run.rate))
	defer ticker.Stop()

	deadline := c.App.Clock.After(run.duration)

	var wg sync.WaitGroup

	sent := 0

	for {
		select {
		case <-ctx.Done():
			wg.Wait()

			return sent
		case <-deadline:
			wg.Wait()

			return sent
		case <-ticker.Chan():
			sent++
			n := sent

			wg.Go(func() { c.post(ctx, client, run, n) })
		}
	}
}

func (c EmailLoadCommand) post(ctx context.Context, client *httpclient.Client, run *loadRun, n int) {
	id := run.prefix + strconv.Itoa(n)
	raw := "From: Load <load@customer.test>\r\nTo: " + run.to + "\r\nSubject: " + id + "\r\n" +
		"Message-ID: <" + strconv.FormatInt(c.App.Clock.Now().UnixNano(), 10) + "." + strconv.Itoa(n) +
		"@customer.test>\r\n\r\nLoad run message " + strconv.Itoa(n) + ".\r\n"

	started := c.App.Clock.Now()
	resp, err := client.New().WithToken(run.secret).WithHeader("Content-Type", "message/rfc822").
		Post(ctx, run.base+"/webhooks/test/inbound?to="+url.QueryEscape(run.to), []byte(raw))
	took := c.App.Clock.Since(started)

	run.mu.Lock()
	defer run.mu.Unlock()

	if err != nil || resp.Status >= 300 {
		run.failed++

		return
	}

	run.accepted = append(run.accepted, took)
}

/*
report waits until every email of the run settled, or the wait ran out,
and prints how long processing took.
*/
func (c EmailLoadCommand) report(
	ctx context.Context, route models.RouteEmailAddressRow, run *loadRun, out *console.Output,
) error {
	q := models.New(c.App.DB)
	give := c.App.Clock.After(settleWait)

	for {
		row, err := q.InboundLatencyBySubject(ctx, models.InboundLatencyBySubjectParams{
			WorkspaceID: route.WorkspaceID, Prefix: run.prefix,
		})
		if err != nil {
			return fmt.Errorf("read load run: %w", err)
		}

		select {
		case <-give:
		case <-c.App.Clock.After(2 * time.Second):
			if row.Settled < row.Total || row.Total == 0 {
				continue
			}
		}

		out.Line("processed %d of %d: p50 %.2fs, p95 %.2fs", row.Settled, row.Total, row.P50Seconds, row.P95Seconds)

		return nil
	}
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}

	sorted := slices.Clone(values)
	slices.Sort(sorted)

	return sorted[int(float64(len(sorted)-1)*p)].Round(time.Millisecond)
}
