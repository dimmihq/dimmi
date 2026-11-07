/*
|--------------------------------------------------------------------------
| Activity Journal
|--------------------------------------------------------------------------
|
| The demo exists so a person can watch what an agent did to a customer's
| account. Every REST request and every MCP tool call is written as one
| plain line: when, through which door, what was asked, for which
| customer, under which idempotency key, and what came of it.
|
*/

package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

/*
entry is one line of the journal. Channel is REST or MCP and Action the
method and path, or the tool name.
*/
type entry struct {
	channel  string
	action   string
	customer string
	key      string
	outcome  string
}

/*
journal writes entries to Out, one line each, never interleaved.
*/
type journal struct {
	mu  sync.Mutex
	out io.Writer
	now func() time.Time
}

func newJournal(out io.Writer, now func() time.Time) *journal {
	return &journal{out: out, now: now}
}

/*
record writes one entry. A write that fails is dropped: the journal is
for watching, and losing a line must never fail the request itself.
*/
func (j *journal) record(e entry) {
	j.mu.Lock()
	defer j.mu.Unlock()

	_, _ = fmt.Fprintf(j.out, "%s  %-4s  %-48s  customer=%-8s  idempotency_key=%s  -> %s\n",
		j.now().Format("15:04:05"), e.channel, e.action, orDash(e.customer), orDash(e.key), e.outcome)
}

/*
line writes free text, used for the startup banner.
*/
func (j *journal) line(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()

	_, _ = fmt.Fprintf(j.out, format+"\n", args...)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}

	return s
}
