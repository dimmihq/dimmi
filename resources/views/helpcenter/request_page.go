/*
|--------------------------------------------------------------------------
| Portal Request Page
|--------------------------------------------------------------------------
|
| One request as its reader sees it: where it stands on the strip from
| submitted to resolved, the fields they see, and the thread.
|
*/

package helpcenter

/*
RequestStep is a stop on a request's progress strip: Done once the
request passed it, Current while it is there.
*/
type RequestStep struct {
	Label   string
	Done    bool
	Current bool
}

/*
RequestPage is one request with its strip, fields and thread.
*/
type RequestPage struct {
	Request

	Description string
	Steps       []RequestStep
	Fields      []RequestField
	Messages    []Message
	Types       []RequestType
	Company     bool
	Error       string
}

/*
Progress returns the strip for a state group: Submitted, We're on it
and Resolved, the current one marked.
*/
func Progress(group string) []RequestStep {
	at := 1

	switch group {
	case "resolved":
		at = 2
	case "submitted":
		at = 0
	}

	labels := []string{"Submitted", "We're on it", "Resolved"}
	out := make([]RequestStep, len(labels))

	for i, l := range labels {
		out[i] = RequestStep{Label: l, Done: i < at || (i == at && at == 2), Current: i == at}
	}

	return out
}
