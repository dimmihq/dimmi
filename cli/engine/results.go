package engine

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
Result is one record's outcome.
*/
type Result struct {
	SourceID string       `json:"source_id"`
	Status   string       `json:"status"`
	ID       string       `json:"id,omitempty"`
	Error    *RecordError `json:"error,omitempty"`
}
