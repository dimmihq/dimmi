/*
|--------------------------------------------------------------------------
| Series Publishing Requests
|--------------------------------------------------------------------------
|
| The inputs of checking and publishing a series, restoring a version,
| reading the report and picking the winner of a split.
|
*/

package requests

import "encoding/json"

/*
CheckSeriesRequest is the input of POST /series/{series}/check: the
spec to check, or the saved working copy when none is sent.
*/
type CheckSeriesRequest struct {
	Series string `path:"series"`
	Body   struct {
		Spec json.RawMessage `json:"spec,omitempty"`
	}
}

/*
PublishSeriesRequest is the input of POST /series/{series}/publications:
a note, and for each removed step people stand on, where they go.
*/
type PublishSeriesRequest struct {
	Series string `path:"series"`
	Body   struct {
		Note  string            `json:"note,omitempty"  maxLength:"280"`
		Moves map[string]string `json:"moves,omitempty"`
	}
}

/*
SeriesVersionRequest names a version of a series.
*/
type SeriesVersionRequest struct {
	Series  string `path:"series"`
	Version string `path:"version"`
}

/*
SeriesReportRequest asks for a series' report over the last days.
*/
type SeriesReportRequest struct {
	Series string `path:"series"`
	Days   int    `query:"days"`
}

/*
SeriesWinnerRequest picks the winning variant of a split step.
*/
type SeriesWinnerRequest struct {
	Series string `path:"series"`
	Node   string `path:"node"`
	Body   struct {
		Path string `json:"path" minLength:"1"`
	}
}
