/*
|--------------------------------------------------------------------------
| Identity Claims
|--------------------------------------------------------------------------
|
| What a verified token says about the customer beyond who they are. A
| claim in the wrong shape is dropped with a warning and the token still
| counts, so one bad avatar address never locks a customer out. An email
| is proven only when the token says email_verified, since many apps never
| confirm addresses and trusting one would hand over that address's
| history. Custom attributes written here count as trusted, the only kind
| agent actions may bind.
|
*/

package identity

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
)

/*
Profile is what a verified token says about its customer: their id in
the host app, and the details given, nil when left out. Attributes holds
custom attributes, a nil value removing one. Warnings name the claims
dropped.
*/
type Profile struct {
	ExternalID    string
	Email         *string
	EmailVerified bool
	Name          *string
	Phone         *string
	AvatarURL     *string
	Locale        *string
	Timezone      *string
	Company       *companies.Fields
	Attributes    map[string]any
	Warnings      []Warning
}

/*
Warning is a claim that was dropped and why.
*/
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	phonePattern     = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
	attributePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

/*
ReadProfile reads the claims a token carries besides the registered
ones.
*/
func ReadProfile(extra map[string]any) Profile {
	r := reader{claims: extra}
	p := Profile{
		Email:     r.email(),
		Name:      r.text("name", 120),
		Phone:     r.phone(),
		AvatarURL: r.avatar(),
		Locale:    r.locale(),
		Timezone:  r.timezone(),
	}
	p.EmailVerified = p.Email != nil && r.flag("email_verified")
	p.Company = r.company()
	p.Attributes = r.attributes(extra["custom_attributes"], "custom_attributes")
	p.Warnings = r.warnings
	return p
}

/*
reader reads claims and collects warnings about the ones it drops.
*/
type reader struct {
	claims   map[string]any
	warnings []Warning
}

func (r *reader) drop(name, reason string) {
	r.warnings = append(r.warnings, Warning{
		Code: "claim_" + name + "_invalid", Message: fmt.Sprintf("Claim %s invalid and ignored: %s.", name, reason),
	})
}

/*
str returns a string claim, warning when it is present but not a
string.
*/
func (r *reader) str(name string) (string, bool) {
	raw, ok := r.claims[name]
	if !ok || raw == nil {
		return "", false
	}
	s, ok := raw.(string)
	if !ok {
		r.drop(name, "it must be a string")
		return "", false
	}
	return strings.TrimSpace(s), true
}

func (r *reader) flag(name string) bool {
	b, _ := r.claims[name].(bool)
	return b
}

func (r *reader) text(name string, limit int) *string {
	s, ok := r.str(name)
	if !ok {
		return nil
	}
	s = strings.Map(func(c rune) rune {
		if unicode.Is(unicode.Bidi_Control, c) || unicode.IsControl(c) {
			return -1
		}
		return c
	}, s)
	if s == "" || len([]rune(s)) > limit {
		r.drop(name, fmt.Sprintf("it must be 1 to %d characters", limit))
		return nil
	}
	return &s
}

func (r *reader) email() *string {
	s, ok := r.str("email")
	if !ok {
		return nil
	}
	parsed, err := mail.ParseAddress(s)
	if err != nil || parsed.Email != s || len(s) > 254 {
		r.drop("email", "it is not an email address")
		return nil
	}
	return contacts.NormalizeEmail(&s)
}

func (r *reader) phone() *string {
	s, ok := r.str("phone")
	if !ok {
		return nil
	}
	if !phonePattern.MatchString(s) {
		r.drop("phone", "it must be in E.164 form, such as +14155550100")
		return nil
	}
	return &s
}

func (r *reader) avatar() *string {
	s, ok := r.str("avatar_url")
	if !ok {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(s) > 2048 {
		r.drop("avatar_url", "it must be an https address")
		return nil
	}
	return &s
}

func (r *reader) locale() *string {
	s, ok := r.str("locale")
	if !ok {
		return nil
	}
	canonical, valid := i18n.Canonical(s)
	if !valid || s == "" {
		r.drop("locale", "it must be a language tag such as en-US")
		return nil
	}
	return &canonical
}

func (r *reader) timezone() *string {
	s, ok := r.str("timezone")
	if !ok {
		return nil
	}
	if _, err := time.LoadLocation(s); err != nil || s == "" || s == "Local" {
		r.drop("timezone", "it must be an IANA time zone such as Europe/Paris")
		return nil
	}
	return &s
}

/*
company reads the company object, which needs an id of its own.
*/
func (r *reader) company() *companies.Fields {
	raw, ok := r.claims["company"]
	if !ok || raw == nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		r.drop("company", "it must be an object")
		return nil
	}
	inner := reader{claims: obj}
	id := inner.text("id", 255)
	if id == nil {
		r.drop("company", "it needs an id")
		return nil
	}
	f := &companies.Fields{ExternalID: id, Name: inner.text("name", 120), Domain: inner.text("domain", 253)}
	f.CustomAttributes = inner.attributes(obj["custom_attributes"], "company")
	r.warnings = append(r.warnings, inner.warnings...)
	return f
}

/*
attributes reads custom attributes: at most 100 keys of lower case
letters, digits and underscores starting with a letter, each a number,
a boolean, a string up to 1,024 characters, or null to remove it.
*/
func (r *reader) attributes(raw any, claim string) map[string]any {
	if raw == nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok || len(obj) > 100 {
		r.drop(claim, "custom_attributes must be an object of at most 100 keys")
		return nil
	}
	out := make(map[string]any, len(obj))
	for k, v := range obj {
		if !attributePattern.MatchString(k) {
			r.drop(claim, fmt.Sprintf("attribute %q needs a lower case name starting with a letter", k))
			continue
		}
		value, ok := scalar(v)
		if !ok {
			r.drop(claim, fmt.Sprintf("attribute %q must be a number, boolean, short string or null", k))
			continue
		}
		out[k] = value
	}
	return out
}

/*
scalar returns a claim value as a custom attribute stores it.
*/
func scalar(v any) (any, bool) {
	switch t := v.(type) {
	case nil, bool:
		return t, true
	case string:
		return t, len([]rune(t)) <= 1024
	case json.Number:
		_, err := t.Float64()
		return t, err == nil
	}
	return nil, false
}
