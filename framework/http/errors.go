/*
|--------------------------------------------------------------------------
| Error Responses
|--------------------------------------------------------------------------
|
| Every failure reaches the client as one JSON shape: a message a person
| can read, a stable code a program can branch on, the broken fields of a
| rejected input, and any details the fault carries. Domain code returns
| faults and validation errors and never picks a status; the status comes
| from the fault's kind here, in one place.
|
| The message follows the locale of the request: the translator the
| context carries looks it up by code as "errors.<code>", with the
| fault's details as arguments, and keeps the original text when no
| locale has one. The code, the field keys and the details never change
| with the locale, so programs keep branching on them.
|
| Anything that is not a fault is a bug or an outage. The client gets a
| plain 500 that leaks nothing, and the error itself goes to the log and
| the error reporter, because nobody else will ever see it.
|
*/

package http

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"net/http"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/validation"
)

//go:embed lang
var messages embed.FS

/*
Messages returns the framework's English error messages, keyed by error
code under the "errors" group, as <locale>/<group>.json files for the
application's translator.
*/
func Messages() fs.FS {
	sub, _ := fs.Sub(messages, "lang")

	return sub
}

/*
The codes the renderer gives errors that carry none of their own.
*/
const (
	CodeValidationFailed = "validation_failed"
	CodeInternal         = "internal_error"
	CodePayloadTooLarge  = "payload_too_large"
)

/*
ResponseError is an error as the client receives it.

Status is the HTTP status. Message is safe to show a person and Code is
stable for programs. Errors maps each rejected field to its messages and
is only set for invalid input. Meta holds the fault's details, written
beside message and code at the top level of the body.
*/
type ResponseError struct {
	Status  int                 `json:"-"`
	Message string              `json:"message"`
	Code    string              `json:"code"`
	Errors  map[string][]string `json:"errors,omitempty"`
	Meta    map[string]any      `json:"-"`
}

/*
Error returns the code and message, for logs.
*/
func (e *ResponseError) Error() string {
	return e.Code + ": " + e.Message
}

/*
GetStatus returns the HTTP status, which lets the API layer send the
error as it is.
*/
func (e *ResponseError) GetStatus() int {
	return e.Status
}

/*
MarshalJSON writes the body with the details beside message and code.
The message, code and field errors always win over a detail of the same
name.
*/
func (e *ResponseError) MarshalJSON() ([]byte, error) {
	body := make(map[string]any, len(e.Meta)+3)
	maps.Copy(body, e.Meta)
	body["message"] = e.Message
	body["code"] = e.Code

	if len(e.Errors) > 0 {
		body["errors"] = e.Errors
	}

	return json.Marshal(body)
}

/*
Errors turns errors into responses.

Report receives every error that is not a fault, after it is logged,
so it can reach an error tracker; it may be nil. Statuses overrides the
status for fault codes whose kind maps to the wrong one, such as a
malformed header that deserves 400 rather than 422.
*/
type Errors struct {
	Report   func(ctx context.Context, err error)
	Statuses map[string]int
}

/*
Render writes the error as the JSON envelope.
*/
func (e Errors) Render(w ResponseWriter, r *Request, err error) {
	res := e.Resolve(r.Context(), err)
	WriteJSON(w, res.Status, res)
}

/*
Resolve returns the response for the error, its message in the locale
of the context. Errors that are not faults are logged and reported here
and come back as a bare 500.
*/
func (e Errors) Resolve(ctx context.Context, err error) *ResponseError {
	if res, ok := errors.AsType[*ResponseError](err); ok {
		return localize(ctx, res)
	}

	if invalid, ok := errors.AsType[*validation.Error](err); ok {
		return &ResponseError{
			Status:  http.StatusUnprocessableEntity,
			Message: summarize(ctx, invalid),
			Code:    CodeValidationFailed,
			Errors:  invalid.Messages(),
		}
	}

	if f, ok := errors.AsType[*fault.Error](err); ok {
		return localize(ctx, &ResponseError{Status: e.status(f), Message: f.Message(), Code: f.Code(), Meta: f.Meta()})
	}

	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return localize(ctx, StatusError(http.StatusRequestEntityTooLarge))
	}

	log.From(ctx).ErrorContext(ctx, "request failed", log.Err(err))

	if e.Report != nil {
		e.Report(ctx, err)
	}

	return localize(ctx, StatusError(http.StatusInternalServerError))
}

/*
localize returns the response with the message its code has in the
locale of the context, or the response as it is when the context
carries no translator, no locale has a message for the code, or the
response lists rejected fields, whose message summarizes them.
*/
func localize(ctx context.Context, res *ResponseError) *ResponseError {
	t := i18n.From(ctx)
	if t == nil || len(res.Errors) > 0 {
		return res
	}

	message, ok := t.Lookup(ctx, "errors."+res.Code, i18n.Args(res.Meta))
	if !ok {
		return res
	}

	localized := *res
	localized.Message = message

	return &localized
}

/*
summarize returns the first message of a failed validation followed by
how many more there are, in the locale of the context.
*/
func summarize(ctx context.Context, invalid *validation.Error) string {
	t := i18n.From(ctx)
	if t == nil || len(invalid.Violations) < 2 {
		return invalid.Message()
	}

	args := i18n.Args{"message": invalid.Violations[0].Message, "count": len(invalid.Violations) - 1}
	if message, ok := t.Lookup(ctx, "validation.summary", args); ok {
		return message
	}

	return invalid.Message()
}

func (e Errors) status(f *fault.Error) int {
	if status, ok := e.Statuses[f.Code()]; ok {
		return status
	}

	if status, ok := queryCodeStatuses()[f.Code()]; ok {
		return status
	}

	return kindStatuses()[f.Kind()]
}

/*
StatusError returns the plain error response for a status, with the
standard code and message for it, as used for unknown routes and
refused methods.
*/
func StatusError(status int) *ResponseError {
	code, ok := statusCodes()[status]
	if !ok {
		code = strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_")
	}

	message := http.StatusText(status) + "."
	if status == http.StatusInternalServerError {
		message = "Server Error."
	}

	return &ResponseError{Status: status, Message: message, Code: code}
}

/*
WriteJSON writes the value as a JSON response with the status. A value
that cannot be encoded is a programming error, so the client gets an
empty 500 instead of a broken body.
*/
func WriteJSON(w ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

/*
kindStatuses maps every fault kind to its status.
*/
func kindStatuses() map[fault.Kind]int {
	return map[fault.Kind]int{
		fault.NotFound:         http.StatusNotFound,
		fault.Conflict:         http.StatusConflict,
		fault.Forbidden:        http.StatusForbidden,
		fault.Unauthenticated:  http.StatusUnauthorized,
		fault.Invalid:          http.StatusUnprocessableEntity,
		fault.Gone:             http.StatusGone,
		fault.Locked:           http.StatusLocked,
		fault.TooLarge:         http.StatusRequestEntityTooLarge,
		fault.RateLimited:      http.StatusTooManyRequests,
		fault.FailedDependency: http.StatusFailedDependency,
		fault.Unavailable:      http.StatusServiceUnavailable,
		fault.Internal:         http.StatusInternalServerError,
	}
}

/*
queryCodeStatuses lists the faults about a malformed list query, which
are bad requests rather than invalid input.
*/
func queryCodeStatuses() map[string]int {
	return map[string]int{
		"invalid_cursor":  http.StatusBadRequest,
		"invalid_filter":  http.StatusBadRequest,
		"invalid_sort":    http.StatusBadRequest,
		"invalid_include": http.StatusBadRequest,
		"invalid_limit":   http.StatusBadRequest,
	}
}

/*
statusCodes names the statuses whose code is not their text in snake
case.
*/
func statusCodes() map[int]string {
	return map[int]string{
		http.StatusUnauthorized:          "unauthenticated",
		http.StatusRequestEntityTooLarge: CodePayloadTooLarge,
		http.StatusUnprocessableEntity:   CodeValidationFailed,
		http.StatusInternalServerError:   CodeInternal,
	}
}
