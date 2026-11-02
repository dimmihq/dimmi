/*
|--------------------------------------------------------------------------
| Domain Faults
|--------------------------------------------------------------------------
|
| Business code says what went wrong, never how to report it. A fault
| carries a kind that every transport maps on its own (an HTTP status, a
| job retry decision, a console exit code), a stable machine code that
| clients may branch on, a message safe to show a person, and optional
| details. A fault is never changed after it is made, so one declared at
| package level can be returned and enriched from any goroutine.
|
*/

package fault

import (
	"errors"
	"maps"
)

/*
Kind is the category of a fault, which decides how transports report it.
*/
type Kind string

/*
The fault kinds. NotFound means the thing does not exist or the caller
may not know it does; Conflict, a clash with current state; Forbidden,
an identified caller lacking permission; Unauthenticated, no valid
identity; Invalid, input that breaks a rule; Gone, something that
existed and was removed; Locked, a resource held by someone else;
TooLarge, input over a size limit; RateLimited, too many attempts;
FailedDependency, a step that depended on another failed one;
Unavailable, a temporary outage worth retrying. Internal is what KindOf
reports for errors that are not faults.
*/
const (
	NotFound         Kind = "not_found"
	Conflict         Kind = "conflict"
	Forbidden        Kind = "forbidden"
	Unauthenticated  Kind = "unauthenticated"
	Invalid          Kind = "invalid"
	Gone             Kind = "gone"
	Locked           Kind = "locked"
	TooLarge         Kind = "too_large"
	RateLimited      Kind = "rate_limited"
	FailedDependency Kind = "failed_dependency"
	Unavailable      Kind = "unavailable"
	Internal         Kind = "internal"
)

/*
Error is a domain fault: its kind, its machine code, its message and
any details attached with With.
*/
type Error struct {
	kind    Kind
	code    string
	message string
	meta    map[string]any
}

/*
New returns a fault of the kind, identified by a stable code such as
"member_exists" and described by a message safe to show a person.
*/
func New(kind Kind, code, message string) *Error {
	return &Error{kind: kind, code: code, message: message}
}

/*
Error returns the code and message, for logs.
*/
func (e *Error) Error() string {
	return e.code + ": " + e.message
}

/*
Kind returns the fault's kind.
*/
func (e *Error) Kind() Kind {
	return e.kind
}

/*
Code returns the fault's machine code.
*/
func (e *Error) Code() string {
	return e.code
}

/*
Message returns the message safe to show a person.
*/
func (e *Error) Message() string {
	return e.message
}

/*
Meta returns a copy of the details attached with With, or nil when there
are none.
*/
func (e *Error) Meta() map[string]any {
	return maps.Clone(e.meta)
}

/*
With returns a copy of the fault carrying one more detail, such as the
limit that was exceeded. The receiver is left unchanged.
*/
func (e *Error) With(key string, value any) *Error {
	meta := make(map[string]any, len(e.meta)+1)
	maps.Copy(meta, e.meta)
	meta[key] = value

	return &Error{kind: e.kind, code: e.code, message: e.message, meta: meta}
}

/*
Is reports whether the target is a fault of the same kind and code, so
errors.Is matches a fault declared once against every copy made from it
with With.
*/
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	if !ok {
		return false
	}

	return e.kind == other.kind && e.code == other.code
}

/*
KindOf returns the kind of the first fault in the error's chain, or
Internal when the chain holds none.
*/
func KindOf(err error) Kind {
	var f *Error
	if !errors.As(err, &f) {
		return Internal
	}

	return f.kind
}

/*
CodeOf returns the code of the first fault in the error's chain, or ""
when the chain holds none.
*/
func CodeOf(err error) string {
	var f *Error
	if !errors.As(err, &f) {
		return ""
	}

	return f.code
}
