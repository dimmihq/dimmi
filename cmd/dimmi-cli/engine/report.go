package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
)

/*
Event is one thing the engine tells the person or agent running it.
It carries ids, kinds and counts, never customer content.
*/
type Event struct {
	Event    string         `json:"event"`
	Time     time.Time      `json:"time"`
	Kind     format.Kind    `json:"kind,omitempty"`
	SourceID string         `json:"source_id,omitempty"`
	Message  string         `json:"message,omitempty"`
	Code     string         `json:"code,omitempty"`
	Counts   map[string]int `json:"counts,omitempty"`
	Data     any            `json:"data,omitempty"`
}

/*
Reporter writes events.
*/
type Reporter interface {
	Report(Event)
}

/*
JSONReporter writes one JSON object per line, for AI agents.
*/
type JSONReporter struct {
	W  io.Writer
	mu sync.Mutex
}

func (r *JSONReporter) Report(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}

	raw, err := json.Marshal(e)
	if err != nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, _ = r.W.Write(append(raw, '\n'))
}

/*
TextReporter writes short lines for people.
*/
type TextReporter struct {
	W       io.Writer
	Verbose bool
	mu      sync.Mutex
}

func (r *TextReporter) Report(e Event) {
	line := textLine(e, r.Verbose)
	if line == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, _ = fmt.Fprintln(r.W, line)
}

func textLine(e Event, verbose bool) string {
	switch e.Event {
	case "kind_started":
		return fmt.Sprintf("→ %s", label(e.Kind))
	case "progress":
		if !verbose {
			return ""
		}

		return fmt.Sprintf("  %s: %s", label(e.Kind), countText(e.Counts))
	case "kind_finished":
		return fmt.Sprintf("✓ %s: %s", label(e.Kind), countText(e.Counts))
	case "record_failed":
		return fmt.Sprintf("  ✗ %s %s: %s %s", e.Kind, e.SourceID, e.Code, e.Message)
	case "warning":
		if e.Kind != "" {
			return fmt.Sprintf("  ! %s: %s", label(e.Kind), e.Message)
		}

		return "  ! " + e.Message
	case "waiting":
		return "  … " + e.Message
	case "error":
		return "✗ " + e.Message
	default:
		if e.Message != "" {
			return e.Message
		}
	}

	return ""
}

func label(kind format.Kind) string { return Label(kind) }

/*
Label is a kind's plural name for people: "companies", "saved replies".
*/
func Label(kind format.Kind) string {
	name := strings.ReplaceAll(string(kind), "_", " ")

	switch {
	case strings.HasSuffix(name, "y") && !strings.HasSuffix(name, "ay") && !strings.HasSuffix(name, "ey"):
		return strings.TrimSuffix(name, "y") + "ies"
	case strings.HasSuffix(name, "s"):
		return name
	}

	return name + "s"
}

func countText(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}

	order := map[string]int{"read": 0, "created": 1, "updated": 2, "unchanged": 3, "skipped": 4, "failed": 5, "files": 6}
	sort.Slice(keys, func(i, j int) bool {
		oi, iok := order[keys[i]]
		oj, jok := order[keys[j]]

		if iok && jok {
			return oi < oj
		}

		if iok != jok {
			return iok
		}

		return keys[i] < keys[j]
	})

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if counts[k] != 0 || k == "read" {
			parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
		}
	}

	return strings.Join(parts, ", ")
}

/*
Discard drops every event.
*/
type Discard struct{}

func (Discard) Report(Event) {}
