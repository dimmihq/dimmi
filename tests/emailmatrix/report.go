//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| Run Report
|--------------------------------------------------------------------------
|
| A run ends as JSON for machines and a table for people, with pass or fail per
| host and how often each host filed mail as spam. A release is held back when
| a host does that twice in a row, so each report is compared with the last.
| Blocked counts against the run: a scenario that never ran proved nothing.
|
*/

package emailmatrix

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"text/tabwriter"
)

const (
	statusPass    = "pass"
	statusFail    = "fail"
	statusBlocked = "blocked"
)

/*
result is how one scenario went on one host.
*/
type result struct {
	Host       string  `json:"host"`
	Scenario   string  `json:"scenario"`
	Status     string  `json:"status"`
	Detail     string  `json:"detail,omitempty"`
	Seconds    float64 `json:"seconds"`
	SpamFolder bool    `json:"spam_folder,omitempty"`
}

/*
report is a whole run.
*/
type report struct {
	Results []result `json:"results"`
}

func resultOf(host, scenario string, err error) result {
	entry := result{Host: host, Scenario: scenario, Status: statusPass}
	if err == nil {
		return entry
	}

	entry.Detail = err.Error()
	entry.Status = statusFail

	if errors.Is(err, errBlocked) {
		entry.Status = statusBlocked
	}

	entry.SpamFolder = errors.Is(err, errSpamFolder)

	return entry
}

func (r report) passed() bool {
	return len(r.Results) > 0 && !slices.ContainsFunc(r.Results, func(entry result) bool {
		return entry.Status != statusPass
	})
}

func (r report) spamHosts() []string {
	var hosts []string

	for _, entry := range r.Results {
		if entry.SpamFolder && !slices.Contains(hosts, entry.Host) {
			hosts = append(hosts, entry.Host)
		}
	}

	return hosts
}

/*
spamTwice names the hosts that filed mail as spam in this run and the last.
*/
func (r report) spamTwice(previous report) []string {
	before := previous.spamHosts()

	return slices.DeleteFunc(r.spamHosts(), func(host string) bool {
		return !slices.Contains(before, host)
	})
}

func (r report) json() ([]byte, error) {
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode report: %w", err)
	}

	return encoded, nil
}

/*
writeTable prints one line per scenario and a summary line per host.
*/
func (r report) writeTable(out io.Writer) error {
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(table, "HOST\tSCENARIO\tSTATUS\tSECONDS\tDETAIL"); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	for _, entry := range r.Results {
		line := fmt.Sprintf(
			"%s\t%s\t%s\t%.1f\t%s", entry.Host, entry.Scenario, entry.Status, entry.Seconds, entry.Detail,
		)
		if _, err := fmt.Fprintln(table, line); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
	}

	if _, err := fmt.Fprintf(table, "\nspam folder: %d host(s)\n", len(r.spamHosts())); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if err := table.Flush(); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}
