/*
|--------------------------------------------------------------------------
| Access Policy Failures
|--------------------------------------------------------------------------
|
| What a workspace's rules answer. A caller who fails a rule is told what
| is required and where to meet it, so every client handles a new rule
| without a change. An admin whose change would shut themselves or every
| owner out is refused before it applies, since by the next request nobody
| could undo it.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Refusals of the access rules.

ErrIPNotAllowed refuses a caller connecting from outside the allowlist
that binds their kind of credential, and carries the address it saw.
ErrSignInMethodRequired refuses a session signed in a way the workspace
does not allow, or through Google from another domain.
ErrReauthenticationRequired asks a session past the workspace's limits
to sign in again.
*/
var (
	ErrIPNotAllowed = fault.New(fault.Forbidden, "ip_not_allowed",
		"Your address is not on this workspace's allowlist. Ask an owner to add it.")
	ErrSignInMethodRequired = fault.New(fault.Forbidden, "sign_in_method_required",
		"This workspace does not allow the way you signed in. Sign in again another way.")
	ErrReauthenticationRequired = fault.New(fault.Forbidden, "reauthentication_required",
		"This workspace asks you to sign in again to continue.")
)

/*
Changes refused because of who they would shut out.

ErrSignInMethodExcludesCaller refuses a list that leaves out the way the
caller signed in. ErrNoOwnerCanSignIn refuses a list no active owner can
meet. ErrIPAllowlistExcludesCaller refuses a dashboard list without the
caller's address, and carries it.
*/
var (
	ErrSignInMethodExcludesCaller = fault.New(fault.Invalid, "sign_in_method_excludes_caller",
		"This change would lock you out: the way you signed in would no longer be allowed.").
		OnField("allowed_sign_in_methods")
	ErrNoOwnerCanSignIn = fault.New(fault.Invalid, "no_owner_can_sign_in",
		"This change would lock everyone out: no owner of the workspace could sign in.").
		OnField("allowed_sign_in_methods")
	ErrIPAllowlistExcludesCaller = fault.New(fault.Invalid, "ip_allowlist_excludes_caller",
		"This change would lock you out: your address is not on the list.").
		OnField("dashboard_ip_allowlist")
)
