package format

import "encoding/json"

/*
Workflow is a source automation, imported disabled. Definition is the
source's own export; Unsupported lists the step types dimmi can't run
yet, so the person rebuilding it knows what to look at.
*/
type Workflow struct {
	Base
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Trigger     string          `json:"trigger,omitempty"`
	Channels    []string        `json:"channels,omitempty"`
	Enabled     bool            `json:"enabled"`
	Steps       []WorkflowStep  `json:"steps"`
	Unsupported []string        `json:"unsupported,omitempty"`
	Definition  json.RawMessage `json:"definition,omitempty"`
}

/*
WorkflowStep is one step of a workflow, in the source's terms, with the
dimmi step it becomes when there is one.
*/
type WorkflowStep struct {
	SourceType string `json:"source_type"`
	Type       string `json:"type,omitempty"`
	Supported  bool   `json:"supported"`
}
