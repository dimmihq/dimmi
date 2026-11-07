/*
|--------------------------------------------------------------------------
| Workflow Definitions
|--------------------------------------------------------------------------
|
| A workflow definition is plain JSON a team can keep in a repository:
| its trigger and the trigger's settings, the channels and audience it
| runs for, the office hours it respects, how often it may run again,
| and a tree of steps. A step may have paths, each holding its own
| steps, read top to bottom.
|
| The same definition always produces the same canonical JSON and so
| the same hash, whatever key order it arrived in. That hash tells the
| working copy from the live version without comparing trees.
|
*/

package workflowspec

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
SchemaVersion is the definition format this package reads.
*/
const SchemaVersion = 1

/*
MaxBytes bounds a stored definition.
*/
const MaxBytes = 256 * 1024

/*
Spec is a workflow definition. Audience is an audience rule; Schedule
and Reentry restrict when the workflow runs; Exclusive stops later
workflows from running for the same event once this one starts.
*/
type Spec struct {
	Schema          int             `json:"schema"`
	Trigger         string          `json:"trigger"`
	TriggerSettings json.RawMessage `json:"trigger_settings,omitempty"`
	Channels        []string        `json:"channels,omitempty"`
	Audience        json.RawMessage `json:"audience,omitempty"`
	Schedule        Schedule        `json:"schedule"`
	Reentry         Reentry         `json:"reentry"`
	Exclusive       bool            `json:"exclusive"`
	Steps           []Step          `json:"steps"`
}

/*
Schedule restricts a workflow to the open or closed hours of a
schedule. Mode is any, open or closed; an empty ScheduleID means the
workspace's default schedule.
*/
type Schedule struct {
	Mode       string `json:"mode"`
	ScheduleID string `json:"schedule_id,omitempty"`
}

/*
Reentry says whether a workflow may run again on a conversation it
already ran on: every time, once per conversation, or after a cooldown.
*/
type Reentry struct {
	Mode     string `json:"mode"`
	Cooldown string `json:"cooldown,omitempty"`
}

/*
Step is one step: its id, its kind, the kind's settings and, for kinds
that branch, its paths.
*/
type Step struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Settings json.RawMessage `json:"settings,omitempty"`
	Paths    []Path          `json:"paths,omitempty"`
}

/*
Path is one way out of a step. User paths carry a label and, on a
branch, a condition, or on a classify step, a description; fixed paths
carry the outcome that takes them, such as typed, timeout or resolved.
*/
type Path struct {
	ID          string          `json:"id"`
	Label       string          `json:"label,omitempty"`
	Outcome     string          `json:"outcome,omitempty"`
	Condition   json.RawMessage `json:"condition,omitempty"`
	Description string          `json:"description,omitempty"`
	Steps       []Step          `json:"steps"`
}

/*
Parse reads a definition, refusing unknown keys so a typo never passes
for a setting that silently does nothing.
*/
func Parse(raw json.RawMessage) (Spec, error) {
	var s Spec
	if len(raw) > MaxBytes {
		return s, &InvalidError{Errors: []Issue{{Code: CodeLimitExceeded, Message: "The definition is over 256 KB."}}}
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&s); err != nil {
		return s, &InvalidError{Errors: []Issue{{Code: CodeSchema, Message: fmt.Sprintf("Not a definition: %v.", err)}}}
	}

	if s.Steps == nil {
		s.Steps = []Step{}
	}

	return s, nil
}

/*
Canonical returns the definition's canonical JSON: keys in a fixed
order at every level, settings and conditions included, no spaces.
*/
func Canonical(s Spec) (json.RawMessage, error) {
	c := s
	var err error

	if c.TriggerSettings, err = canonicalRaw(c.TriggerSettings); err != nil {
		return nil, fmt.Errorf("canonical trigger settings: %w", err)
	}

	if c.Audience, err = canonicalRaw(c.Audience); err != nil {
		return nil, fmt.Errorf("canonical audience: %w", err)
	}

	if c.Steps, err = canonicalSteps(c.Steps); err != nil {
		return nil, err
	}

	out, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode definition: %w", err)
	}

	return out, nil
}

/*
Hash returns the hex SHA-256 of the canonical JSON.
*/
func Hash(s Spec) (string, error) {
	raw, err := Canonical(s)
	if err != nil {
		return "", err
	}

	return hashing.SHA256Hex(raw), nil
}

/*
canonicalSteps canonicalizes the raw parts of every step and path.
*/
func canonicalSteps(steps []Step) ([]Step, error) {
	out := make([]Step, len(steps))
	for i, step := range steps {
		settings, err := canonicalRaw(step.Settings)
		if err != nil {
			return nil, fmt.Errorf("canonical settings of %s: %w", step.ID, err)
		}

		step.Settings = settings
		if step.Paths, err = canonicalPaths(step.Paths); err != nil {
			return nil, err
		}

		out[i] = step
	}

	return out, nil
}

/*
canonicalPaths canonicalizes the conditions and steps of paths.
*/
func canonicalPaths(paths []Path) ([]Path, error) {
	if paths == nil {
		return nil, nil
	}

	out := make([]Path, len(paths))
	for j, p := range paths {
		var err error
		if p.Condition, err = canonicalRaw(p.Condition); err != nil {
			return nil, fmt.Errorf("canonical condition of %s: %w", p.ID, err)
		}

		if p.Steps, err = canonicalSteps(p.Steps); err != nil {
			return nil, err
		}

		out[j] = p
	}

	return out, nil
}

/*
canonicalRaw re-encodes raw JSON with sorted object keys; empty objects
and null become absent, so "{}" and a missing setting hash the same.
*/
func canonicalRaw(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" || string(trimmed) == "{}" {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	return out, nil
}
