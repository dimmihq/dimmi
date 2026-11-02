/*
|--------------------------------------------------------------------------
| Responses
|--------------------------------------------------------------------------
|
| A response is plain data: status, headers and the body, already read
| and within the size cap, so callers never have to remember to close
| anything. A streamed response is the exception; its body is read as it
| arrives and must be closed.
|
*/

package httpclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

/*
Response is a received response, or the canned answer of a fake stub.
Status zero in a stub means 200. Body is empty for a streamed response,
whose body comes from Reader.
*/
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	stream io.ReadCloser
}

/*
JSON decodes the body into the value.
*/
func (r *Response) JSON(v any) error {
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("decode json response: %w", err)
	}

	return nil
}

/*
Reader returns the body as it arrives for a streamed response, or the
read body otherwise.
*/
func (r *Response) Reader() io.Reader {
	if r.stream != nil {
		return r.stream
	}

	return bytes.NewReader(r.Body)
}

/*
Close releases a streamed response. It does nothing for a read one.
*/
func (r *Response) Close() error {
	if r.stream == nil {
		return nil
	}

	if err := r.stream.Close(); err != nil {
		return fmt.Errorf("close response: %w", err)
	}

	return nil
}

/*
streamBody ends the request's timeout when the caller closes the body,
and stops yielding data once the request's context ends, even data the
transport already buffered, so a canceled stream fails the same way
whether its bytes are still on the wire or not.
*/
type streamBody struct {
	io.ReadCloser
	cancel func()
	ended  func() error
}

/*
Read reads from the body until the request's context ends.
*/
func (s *streamBody) Read(p []byte) (int, error) {
	if err := s.ended(); err != nil {
		return 0, fmt.Errorf("read stream: %w", err)
	}

	n, err := s.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, fmt.Errorf("read stream: %w", err)
	}

	return n, err
}

/*
Close closes the body and ends the request.
*/
func (s *streamBody) Close() error {
	defer s.cancel()

	return s.ReadCloser.Close()
}
