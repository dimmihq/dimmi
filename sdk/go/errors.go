package dimmi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

/*
Error is an error answer from the API: its HTTP status, the stable code
to branch on, a message for people, and per-field problems on a
validation error. Match a kind with errors.Is(err, dimmi.ErrNotFound).
*/
type Error struct {
	Status     int
	Code       string
	Message    string
	Fields     map[string][]string
	RetryAfter time.Duration
	RequestID  string
	Body       []byte
}

func (e *Error) Error() string {
	return fmt.Sprintf("dimmi: %d %s: %s", e.Status, e.Code, e.Message)
}

// Is matches the sentinel kinds below by status.
func (e *Error) Is(target error) bool {
	k, ok := target.(kind)
	if !ok {
		return false
	}
	switch k {
	case ErrBadRequest:
		return e.Status == http.StatusBadRequest
	case ErrUnauthorized:
		return e.Status == http.StatusUnauthorized
	case ErrForbidden:
		return e.Status == http.StatusForbidden
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case ErrConflict:
		return e.Status == http.StatusConflict
	case ErrValidation:
		return e.Status == http.StatusUnprocessableEntity
	case ErrRateLimited:
		return e.Status == http.StatusTooManyRequests
	case ErrServer:
		return e.Status >= 500
	}
	return false
}

type kind string

func (k kind) Error() string { return string(k) }

// Kinds of API error, for errors.Is.
const (
	ErrBadRequest   kind = "dimmi: bad request"
	ErrUnauthorized kind = "dimmi: unauthorized"
	ErrForbidden    kind = "dimmi: forbidden"
	ErrNotFound     kind = "dimmi: not found"
	ErrConflict     kind = "dimmi: conflict"
	ErrValidation   kind = "dimmi: validation failed"
	ErrRateLimited  kind = "dimmi: rate limited"
	ErrServer       kind = "dimmi: server error"
)

// readError reads and closes an error answer.
func readError(res *http.Response) *Error {
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	e := &Error{Status: res.StatusCode, Body: raw, RequestID: res.Header.Get("X-Request-Id")}
	var body struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
		Fields  map[string][]string `json:"fields"`
	}
	if json.Unmarshal(raw, &body) == nil {
		e.Code, e.Message, e.Fields = body.Code, body.Message, body.Errors
		if len(e.Fields) == 0 {
			e.Fields = body.Fields
		}
	}
	if e.Code == "" {
		e.Code = "http_" + strconv.Itoa(res.StatusCode)
	}
	if e.Message == "" {
		e.Message = http.StatusText(res.StatusCode)
	}
	e.RetryAfter = retryAfter(res.Header.Get("Retry-After"))
	return e
}

func retryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil && s >= 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}
