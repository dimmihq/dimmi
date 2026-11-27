/*
|--------------------------------------------------------------------------
| Forwarded Lines
|--------------------------------------------------------------------------
|
| A logger can hand its lines to a second handler, such as the error
| tracker's, beside the process output, which stays exactly as it was.
| The copy leaves the process, so it is held to a stricter rule than the
| output: secrets are hidden as on every line, attributes that carry a
| message's recipients, subject, links or content are dropped, and email
| addresses are blanked wherever they appear.
|
*/

package log

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
)

/*
unforwarded names the attributes never sent off the process: the mail
fallback's recipients, subject, links and raw message, and any message
or request body or email address.
*/
func unforwarded() []string {
	return []string{"recipients", "subject", "links", "message", "body", "email", "request_body"}
}

/*
emailAddress matches an email address in free text.
*/
var emailAddress = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

/*
blankEmails replaces every email address in the text.
*/
func blankEmails(text string) string {
	return emailAddress.ReplaceAllString(text, "[email]")
}

/*
tee writes each record to the primary handler, at its own level, and to
the forwarded handler, at its own. Only the primary's error is returned:
a forwarded copy that fails never fails the line.
*/
type tee struct {
	primary slog.Handler
	forward slog.Handler
}

/*
Enabled reports whether either handler takes records at the level.
*/
func (t tee) Enabled(ctx context.Context, level slog.Level) bool {
	return t.primary.Enabled(ctx, level) || t.forward.Enabled(ctx, level)
}

/*
Handle writes the record to each handler that takes its level.
*/
func (t tee) Handle(ctx context.Context, record slog.Record) error {
	var err error
	if t.primary.Enabled(ctx, record.Level) {
		err = t.primary.Handle(ctx, record)
	}

	if t.forward.Enabled(ctx, record.Level) {
		_ = t.forward.Handle(ctx, record.Clone())
	}

	return err
}

/*
WithAttrs adds the attributes on both sides.
*/
func (t tee) WithAttrs(attrs []slog.Attr) slog.Handler {
	return tee{primary: t.primary.WithAttrs(attrs), forward: t.forward.WithAttrs(attrs)}
}

/*
WithGroup opens the group on both sides.
*/
func (t tee) WithGroup(name string) slog.Handler {
	return tee{primary: t.primary.WithGroup(name), forward: t.forward.WithGroup(name)}
}

/*
scrubbed hands the forwarded handler records with secrets hidden, the
unforwarded attributes dropped and email addresses blanked.
*/
type scrubbed struct {
	next   slog.Handler
	redact func(groups []string, attr slog.Attr) slog.Attr
	drop   map[string]struct{}
	groups []string
}

/*
newScrubbed wraps the handler, hiding the extra secret names too.
*/
func newScrubbed(next slog.Handler, extra []string) scrubbed {
	drop := map[string]struct{}{}
	for _, name := range unforwarded() {
		drop[normalize(name)] = struct{}{}
	}

	return scrubbed{next: next, redact: redactor(extra), drop: drop}
}

/*
Enabled defers to the forwarded handler.
*/
func (s scrubbed) Enabled(ctx context.Context, level slog.Level) bool {
	return s.next.Enabled(ctx, level)
}

/*
Handle rebuilds the record from its scrubbed message and attributes.
*/
func (s scrubbed) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, blankEmails(record.Message), record.PC)
	record.Attrs(func(a slog.Attr) bool {
		if a, ok := s.attr(s.groups, a); ok {
			clean.AddAttrs(a)
		}

		return true
	})

	return s.next.Handle(ctx, clean) //nolint:wrapcheck // the forwarded handler's error is dropped by the tee
}

/*
WithAttrs scrubs the attributes before the forwarded handler keeps them.
*/
func (s scrubbed) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if a, ok := s.attr(s.groups, a); ok {
			clean = append(clean, a)
		}
	}

	next := s
	next.next = s.next.WithAttrs(clean)

	return next
}

/*
WithGroup opens the group on the forwarded handler.
*/
func (s scrubbed) WithGroup(name string) slog.Handler {
	next := s
	next.next = s.next.WithGroup(name)
	next.groups = append(slices.Clone(s.groups), name)

	return next
}

/*
attr returns the attribute as it may leave the process, or false when
it may not.
*/
func (s scrubbed) attr(groups []string, a slog.Attr) (slog.Attr, bool) {
	if _, ok := s.drop[normalize(a.Key)]; ok {
		return slog.Attr{}, false
	}

	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		inner := slices.Concat(groups, []string{a.Key})
		var kept []slog.Attr
		for _, g := range a.Value.Group() {
			if g, ok := s.attr(inner, g); ok {
				kept = append(kept, g)
			}
		}

		return slog.GroupAttrs(a.Key, kept...), true
	}

	a = s.redact(groups, a)

	switch a.Value.Kind() { //nolint:exhaustive // numbers, times and booleans carry no text to scrub
	case slog.KindString:
		return slog.String(a.Key, blankEmails(a.Value.String())), true
	case slog.KindAny:
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, blankEmails(err.Error())), true
		}

		return slog.String(a.Key, blankEmails(fmt.Sprintf("%+v", a.Value.Any()))), true
	default:
		return a, true
	}
}
