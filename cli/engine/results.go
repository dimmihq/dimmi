package engine

import "github.com/dimmihq/dimmi/cli/format"

/*
RecordError is why dimmi refused a record. Messages never hold the
record's content.
*/
type RecordError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

/*
The outcomes of one record.
*/
const (
	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
	StatusDeferred  = "deferred"
	StatusFailed    = "failed"
)

/*
Ref names a record by its kind and source id.
*/
type Ref struct {
	Kind     format.Kind `json:"kind"`
	SourceID string      `json:"source_id"`
}

/*
Result is one record's outcome. A deferred record names the records it
waits for.
*/
type Result struct {
	SourceID   string       `json:"source_id"`
	Status     string       `json:"status"`
	ID         string       `json:"id,omitempty"`
	Error      *RecordError `json:"error,omitempty"`
	WaitingFor []Ref        `json:"waiting_for,omitempty"`
}
