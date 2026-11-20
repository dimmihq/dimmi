/*
|--------------------------------------------------------------------------
| WhatsApp Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a workspace connects a WhatsApp number or
| writes to a customer on one. Each says what to fix: a token Meta
| refused or one without the WhatsApp permissions, an account or number
| the token cannot see, a signup that ran too long, a window that closed,
| or a template that cannot be sent.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
WhatsApp failures.

ErrWhatsAppTokenRejected is a token Meta refused or one lacking the
WhatsApp permissions. ErrWhatsAppAccountNotFound is an account the token
cannot see, ErrWhatsAppNumberNotFound a number not in it.
ErrWhatsAppSignupExpired is a signup code Meta no longer takes.
ErrWhatsAppSignupUnavailable is a popup this install has no Meta app
for. ErrChannelWindowClosed refuses free text more than 24 hours after
the customer last wrote. ErrWhatsAppTemplateUnavailable is a template
that is not approved or not sendable from the inbox, and
ErrWhatsAppTemplateValueMissing one with a variable left empty.
ErrWhatsAppUnavailable is Meta not answering. ErrWhatsAppTemplateInvalid
is a template a teammate wrote that Meta would not take; its field and
message say what to change. ErrWhatsAppTemplateRefused is one Meta
refused, in Meta's words.
*/
var (
	ErrWhatsAppTokenRejected = fault.New(fault.Invalid, "whatsapp_token_rejected",
		"Meta refused this token, or it lacks the WhatsApp permissions.").OnField("access_token")
	ErrWhatsAppAccountNotFound = fault.New(fault.Invalid, "whatsapp_account_not_found",
		"This token cannot see a WhatsApp account with this id.").OnField("waba_id")
	ErrWhatsAppNumberNotFound = fault.New(fault.Invalid, "whatsapp_number_not_found",
		"This number is not on the WhatsApp account.").OnField("phone_number_id")
	ErrWhatsAppSignupExpired = fault.New(fault.Invalid, "whatsapp_signup_expired",
		"The Meta signup expired. Connect again.").OnField("code")
	ErrWhatsAppSignupUnavailable = fault.New(fault.Invalid, "whatsapp_signup_unavailable",
		"This server has no Meta app for the WhatsApp popup. Connect with your own Meta app.")
	ErrChannelWindowClosed = fault.New(fault.Invalid, "channel_window_closed",
		"More than 24 hours passed since the customer's last message. Send an approved template.")
	ErrWhatsAppTemplateUnavailable = fault.New(fault.Invalid, "whatsapp_template_unavailable",
		"Meta has not approved this template, or dimmi cannot send its kind of template yet.").OnField("template")
	ErrWhatsAppTemplateValueMissing = fault.New(fault.Invalid, "whatsapp_template_value_missing",
		"Fill in every variable of the template.").OnField("values")
	ErrWhatsAppUnavailable = fault.New(fault.FailedDependency, "whatsapp_unavailable",
		"Meta could not be reached. Try again shortly.")
	ErrWhatsAppTemplateInvalid = fault.New(fault.Invalid, "whatsapp_template_invalid",
		"Meta would not take this template as written.")
	ErrWhatsAppTemplateRefused = fault.New(fault.Invalid, "whatsapp_template_refused",
		"Meta refused this template.")
)

/*
WhatsAppTemplateInvalid is a written template's problem on one field.
*/
func WhatsAppTemplateInvalid(field, message string) *fault.Error {
	return fault.New(fault.Invalid, ErrWhatsAppTemplateInvalid.Code(), message).OnField(field)
}

/*
WhatsAppTemplateRefused is Meta's refusal of a written template, in its
own words when it gave some.
*/
func WhatsAppTemplateRefused(message string) *fault.Error {
	if message == "" {
		return ErrWhatsAppTemplateRefused
	}

	return fault.New(fault.Invalid, ErrWhatsAppTemplateRefused.Code(), message)
}
