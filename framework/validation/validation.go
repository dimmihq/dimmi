/*
|--------------------------------------------------------------------------
| Validation
|--------------------------------------------------------------------------
|
| Input is checked against rules written in the `validate` tag of each
| struct field, and every broken rule is reported at once, so a client
| fixes a form in one round trip instead of one error at a time. Fields
| are named by their JSON names, the same names the client sent.
|
|   Name   string   `json:"name"   validate:"required,max=255"`
|   Emails []string `json:"emails" validate:"required,dive,email"`
|
| Messages come from the translator in the locale of the context, one
| key per rule such as "validation.required", with {{.field}} and
| {{.param}} filled in; size rules pick a key per kind of value, such as
| "validation.min.string". The framework ships the English messages; the
| application rewords or translates them in its own lang files. Rule
| codes never change with the locale, only messages do.
|
| Rules the factory lacks are added as Rule values, each carrying its own
| English message for when no locale has one. Rules are registered while
| the application boots, before the first validation, because the rule
| table is not safe to change while it is in use.
|
*/

package validation

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/i18n"
)

//go:embed lang
var messages embed.FS

/*
Errors a validation fails with when the code, not the input, is wrong.

ErrNotStruct is returned when the value given to Struct is not a struct
or a pointer to one. ErrInvalidRule means a tag names a rule that does
not exist, gives a parameter the rule cannot read, or applies a rule to
a value it cannot judge, such as email on a number.
*/
var (
	ErrNotStruct   = errors.New("validation: value is not a struct")
	ErrInvalidRule = errors.New("validation: invalid rule")
)

/*
Rule is a check the factory does not provide, such as a prefixed public
ID. Its name is the tag that applies it: a rule named "public_id" is used
as `validate:"public_id=usr"`, and "usr" reaches it as the parameter.
*/
type Rule interface {
	/*
		Name returns the tag that applies the rule.
	*/
	Name() string

	/*
		Message returns the English message for a value that breaks the
		rule, used when no locale has a "validation.<name>" message.
		{field} stands for the field path and {param} for the parameter.
	*/
	Message() string

	/*
		Passes reports whether the value satisfies the rule. The value is
		the field's own value, with pointers already followed.
	*/
	Passes(ctx context.Context, value any, param string) bool
}

/*
Factory validates structs. It is safe for concurrent use once every rule
is registered.
*/
type Factory struct {
	builtins   map[string]check
	rules      map[string]Rule
	types      sync.Map
	clock      clock.Clock
	translator *i18n.Translator
}

/*
Option configures a factory.
*/
type Option func(f *Factory)

/*
WithTranslator renders messages with the translator, which should hold
the messages of this package beside the application's own. Without it,
messages are the framework's English ones.
*/
func WithTranslator(t *i18n.Translator) Option {
	return func(f *Factory) {
		f.translator = t
	}
}

/*
WithClock tells the present time to the rules that compare a time with
it, such as gt on a time. Without it the system clock is used.
*/
func WithClock(c clock.Clock) Option {
	return func(f *Factory) {
		f.clock = c
	}
}

/*
Messages returns the framework's English validation messages as
<locale>/<group>.json files, for loading into the application's
translator.
*/
func Messages() fs.FS {
	sub, _ := fs.Sub(messages, "lang")

	return sub
}

/*
New returns a factory with the built-in rules.
*/
func New(options ...Option) (*Factory, error) {
	english, err := i18n.New(i18n.Config{Locale: i18n.Source}, Messages())
	if err != nil {
		return nil, fmt.Errorf("load validation messages: %w", err)
	}

	f := &Factory{builtins: builtins(), rules: map[string]Rule{}, clock: clock.System(), translator: english}
	for _, option := range options {
		option(f)
	}

	return f, nil
}

/*
Register adds a rule, replacing any rule of the same name, the built-in
ones included. A name that is empty or steers the walk, such as "dive",
is refused.
*/
func (f *Factory) Register(rule Rule) error {
	name := rule.Name()
	if name == "" || strings.ContainsAny(name, ",=") ||
		name == tagOmitEmpty || name == tagDive || name == tagKeys || name == tagEndKeys {
		return fmt.Errorf("register rule %q: %w", name, ErrInvalidRule)
	}

	f.rules[name] = rule

	return nil
}

/*
typeName names the type of a value, as %T would, without the value
itself ever reaching an error message, since validated values include
passwords and tokens.
*/
func typeName(value any) string {
	if value == nil {
		return "<nil>"
	}

	return reflect.TypeOf(value).String()
}

/*
Struct checks every field of the struct, nested structs and collection
items included. It returns nil when all rules pass and an *Error listing
each broken rule otherwise.
*/
func (f *Factory) Struct(ctx context.Context, value any) error {
	name := typeName(value)
	v, given := follow(reflect.ValueOf(value))
	if !given || v.Kind() != reflect.Struct {
		return fmt.Errorf("validate %s: %w", name, ErrNotStruct)
	}

	w := &walker{factory: f}
	if err := w.walkStruct(ctx, v, ""); err != nil {
		return fmt.Errorf("validate %s: %w", name, err)
	}

	if len(w.broken) == 0 {
		return nil
	}

	violations := make([]Violation, 0, len(w.broken))
	for _, b := range w.broken {
		b.violation.Message = f.message(ctx, b)
		violations = append(violations, b.violation)
	}

	return &Error{Violations: violations}
}

/*
run applies one rule to a value: a registered rule first, so one can
replace a built-in rule, then a built-in rule.
*/
func (f *Factory) run(ctx context.Context, v reflect.Value, s step) (bool, error) {
	if rule, ok := f.rules[s.rule]; ok {
		var value any
		if v.CanInterface() {
			value = v.Interface()
		}

		return rule.Passes(ctx, value, s.param), nil
	}

	if check, ok := f.builtins[s.rule]; ok {
		return check(f, v, s.param)
	}

	return false, fmt.Errorf("%w: no rule named %q", ErrInvalidRule, s.rule)
}

/*
message renders the message of a broken rule in the locale of the
context: the rule's message for the kind of value, then its plain
message, then a registered rule's own English message, and finally the
generic "validation.invalid". A numeric parameter is also the count
that picks a plural form.
*/
func (f *Factory) message(ctx context.Context, b broken) string {
	v := b.violation
	args := i18n.Args{"field": v.Field, "param": v.Param}

	if _, err := strconv.ParseFloat(v.Param, 64); err == nil {
		args["count"] = v.Param
	}

	for _, key := range []string{"validation." + v.Rule + "." + kind(b.kind), "validation." + v.Rule} {
		if message, ok := f.translator.Lookup(ctx, key, args); ok {
			return message
		}
	}

	if rule, ok := f.rules[v.Rule]; ok {
		return strings.NewReplacer("{field}", v.Field, "{param}", v.Param).Replace(rule.Message())
	}

	return f.translator.T(ctx, "validation.invalid", args)
}

/*
kind names the kind of value a size rule measures: the length of a
string, the value of a number, or the items of a collection.
*/
func kind(k reflect.Kind) string {
	if k == reflect.String {
		return "string"
	}

	if k == reflect.Slice || k == reflect.Array || k == reflect.Map {
		return "array"
	}

	if k >= reflect.Int && k <= reflect.Float64 {
		return "numeric"
	}

	return "other"
}
