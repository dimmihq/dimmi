/*
|--------------------------------------------------------------------------
| Identity Binding
|--------------------------------------------------------------------------
|
| The model never says whose account a call acts on. Contact values come
| only from a context that proves who the customer is: a widget message
| sent from a session signed in with a verified identity token, or a
| message the workspace's own server posted through the API. Anywhere
| else no contact value resolves, so an action that binds one is simply
| not offered, and a prompt-injected customer can only ever act on their
| own account. Within a verified context an email counts only when
| verified, and a custom attribute only when a trusted source wrote it.
|
*/

package actionkit

import (
	"encoding/json"
	"slices"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
Identity is what a run knows about who the customer is: whether the
context proves it, whether it proves only their email, whether an
anonymous visitor could prove their email with a code, and the contact.
*/
type Identity struct {
	Verified    bool
	EmailProven bool
	EmailCodes  bool
	Contact     *models.Contact
}

/*
VerifiedContext reports whether the message that triggered a run proves
who the customer is: stamped verified by the widget, or posted on the
API channel by the workspace itself.
*/
func VerifiedContext(channelType string, trigger models.Message) bool {
	if trigger.AuthorType != "contact" {
		return false
	}
	switch channelType {
	case "api":
		return true
	case "widget":
		var meta struct {
			Widget struct {
				IdentityVerified bool `json:"identity_verified"`
			} `json:"widget"`
		}
		return json.Unmarshal(trigger.Metadata, &meta) == nil && meta.Widget.IdentityVerified
	}
	return false
}

/*
EmailContext reports whether the message that triggered a run came from
a messenger session whose visitor recently proved their email with a
code, and whether one that did not could still prove it.
*/
func EmailContext(channelType string, trigger models.Message) (proven, codes bool) {
	if trigger.AuthorType != "contact" || channelType != "widget" {
		return false, false
	}
	var meta struct {
		Widget struct {
			EmailVerified bool `json:"email_verified"`
		} `json:"widget"`
	}
	if json.Unmarshal(trigger.Metadata, &meta) != nil {
		return false, true
	}
	return meta.Widget.EmailVerified, !meta.Widget.EmailVerified
}

/*
ContactValues are the contact values a verified context may bind:
external id, a verified email, and the custom attributes a trusted
source wrote. A context that proves only the email binds only that.
Outside a verified context there are none.
*/
func ContactValues(id Identity) map[string]any {
	out := map[string]any{}
	c := id.Contact
	if c == nil || c.DeletedAt != nil {
		return out
	}
	if !id.Verified {
		if id.EmailProven && c.Email != nil && *c.Email != "" && c.EmailVerified {
			out[actiontemplate.ContactEmail] = *c.Email
		}
		return out
	}
	if c.ExternalID != nil && *c.ExternalID != "" {
		out[actiontemplate.ContactExternalID] = *c.ExternalID
	}
	if c.Email != nil && *c.Email != "" && c.EmailVerified {
		out[actiontemplate.ContactEmail] = *c.Email
	}
	attrs := map[string]any{}
	all := map[string]any{}
	_ = json.Unmarshal(c.CustomAttributes, &all)
	for k, v := range all {
		if slices.Contains(c.TrustedAttributeKeys, k) && v != nil {
			attrs[k] = v
		}
	}
	out[actiontemplate.ContactAttr] = attrs
	return out
}

/*
bindable reports whether every contact value the action binds resolves.
*/
func bindable(s Spec, contact map[string]any) bool {
	vars := actiontemplate.Vars{Contact: contact}
	for _, r := range s.Refs() {
		if !r.UsesContact() {
			continue
		}
		if _, ok := vars.Lookup(r); !ok {
			return false
		}
	}
	return true
}

/*
Bindings are the frozen values a call is bound to: the contact values
it used. They stay on the server, so what runs after a confirmation or
an approval is exactly what was bound when the call was proposed.
*/
type Bindings struct {
	Contact map[string]any `json:"contact"`
}

/*
bindingsFor keeps only the contact values the action uses.
*/
func bindingsFor(s Spec, contact map[string]any) Bindings {
	out := Bindings{Contact: map[string]any{}}
	vars := actiontemplate.Vars{Contact: contact}
	attrs := map[string]any{}
	for _, r := range s.Refs() {
		value, ok := vars.Lookup(r)
		switch {
		case !r.UsesContact() || !ok:
		case r.Path[0] == actiontemplate.ContactAttr:
			attrs[r.Path[1]] = value
		default:
			out.Contact[r.Path[0]] = value
		}
	}
	if len(attrs) > 0 {
		out.Contact[actiontemplate.ContactAttr] = attrs
	}
	return out
}
