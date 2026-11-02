/*
|--------------------------------------------------------------------------
| Server-Sent Events
|--------------------------------------------------------------------------
|
| Streaming APIs answer with server-sent events: blocks of field lines
| separated by blank lines. The stream itself may run for a long time, so
| it has no total size cap, but a single line or event is capped, which
| keeps a server that never ends an event from growing memory without
| bound.
|
*/

package httpclient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

/*
MaxEventSize caps one line, and the data of one event, in bytes.
*/
const MaxEventSize = 1 << 20

/*
Event is one server-sent event. Event is the event type, "message" when
the server names none. Data joins the event's data lines with newlines.
ID is the last event ID the server set, carried over from earlier
events.
*/
type Event struct {
	ID    string
	Event string
	Data  string
}

/*
SSEReader reads events from a streamed response.
*/
type SSEReader struct {
	scanner *bufio.Scanner
	lastID  string
}

/*
SSE returns a reader over the response's events. Request the response
with Stream and close it when done.
*/
func SSE(resp *Response) *SSEReader {
	scanner := bufio.NewScanner(resp.Reader())
	scanner.Buffer(make([]byte, 0, 4096), MaxEventSize)

	return &SSEReader{scanner: scanner}
}

/*
Next returns the next event, or io.EOF once the stream ends. An event
cut off by the end of the stream is dropped, as the protocol requires.
*/
func (s *SSEReader) Next() (Event, error) {
	event := Event{}

	var data strings.Builder

	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" && data.Len() > 0 {
			return s.dispatch(event, data.String()), nil
		}

		if line == "" {
			event = Event{}

			continue
		}

		name, value := field(line)
		switch name {
		case "data":
			data.WriteString(value + "\n")
		case "event":
			event.Event = value
		case "id":
			s.lastID = value
		}

		if data.Len() > MaxEventSize {
			return Event{}, fmt.Errorf("read event: %w", ErrTooLarge)
		}
	}

	if err := s.scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return Event{}, fmt.Errorf("read event: %w", ErrTooLarge)
		}

		return Event{}, fmt.Errorf("read event: %w", err)
	}

	return Event{}, io.EOF
}

func (s *SSEReader) dispatch(event Event, data string) Event {
	event.ID = s.lastID
	event.Data = strings.TrimSuffix(data, "\n")

	if event.Event == "" {
		event.Event = "message"
	}

	return event
}

/*
field splits a line into its field name and value, dropping the one
space that may follow the colon. Comment lines have an empty name.
*/
func field(line string) (string, string) {
	name, value, _ := strings.Cut(line, ":")

	return name, strings.TrimPrefix(value, " ")
}
