/*
|--------------------------------------------------------------------------
| Widget Errors
|--------------------------------------------------------------------------
|
| What can go wrong for a visitor's messenger or for whoever configures
| one. The codes are public: the widget and every SDK branch on them, so a
| code never changes once shipped. A missing app and a malformed one are the
| same answer, so app ids cannot be probed.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Failures reaching a widget.
*/
var (
	ErrAppNotFound    = fault.New(fault.NotFound, "app_not_found", "No messenger has this app id.")
	ErrWidgetDisabled = fault.New(fault.Forbidden, "widget_disabled",
		"This messenger is turned off.")
	ErrOriginNotAllowed = fault.New(fault.Forbidden, "origin_not_allowed",
		"This site is not allowed to use this messenger.")
	ErrAnonymousNotAllowed = fault.New(fault.Forbidden, "anonymous_not_allowed",
		"This messenger only works for signed-in visitors.")
	ErrWidgetUnavailable = fault.New(fault.Unavailable, "widget_unavailable",
		"The messenger is turned off on this install.")
)

/*
Failures of settings.
*/
var (
	ErrEntitlementLimitReached = fault.New(fault.Invalid, "entitlement_limit_reached",
		"This workspace has reached its limit.")
	ErrBrandingLocked = fault.New(fault.Forbidden, "branding_locked",
		"Badge hides once workspace pays any invoice.")
)

/*
WidgetField returns the refusal of a settings field, naming the field and
why, with the code forms show field errors under.
*/
func WidgetField(field, message string) *fault.Error {
	return fault.New(fault.Invalid, "validation_failed", message).OnField(field)
}
