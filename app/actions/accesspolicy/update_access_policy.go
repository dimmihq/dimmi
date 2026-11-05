/*
|--------------------------------------------------------------------------
| Update Access Policy
|--------------------------------------------------------------------------
|
| Changing who may work in a workspace can shut people out, the caller
| included, so a change is weighed against the people it binds before it
| applies. A list that leaves out how the caller signed in, leaves out
| their address, or that no owner of the workspace could meet is refused
| with what would have happened. A shorter session limit is never refused:
| the worst it does is ask the caller to sign in again.
|
| Limits only ever shorten what the install allows. A change lands in the
| audit log with its values, tells the other owners, and closes the
| workspace's open connections so it bites on the next thing they do.
|
*/

package accesspolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
managePolicy is the permission to change a workspace's rules.
*/
const managePolicy = "security_policy:manage"

/*
Change is what a request changes; a nil field keeps the stored value. A
limit is cleared by ClearSessionMaxAge or ClearSessionIdleTimeout, since
nil already means keep.
*/
type Change struct {
	AllowedSignInMethods    *[]string
	GoogleAllowedDomains    *[]string
	SessionMaxAge           *int
	ClearSessionMaxAge      bool
	SessionIdleTimeout      *int
	ClearSessionIdleTimeout bool
	DashboardAllowlist      *[]models.AllowedAddress
	APIAllowlist            *[]models.AllowedAddress
}

/*
UpdateAccessPolicyInput is who changes the rules, through their
membership of the workspace, and what changes.
*/
type UpdateAccessPolicyInput struct {
	Principal  *appauth.Principal
	Membership authz.Membership
	Change     Change
}

/*
pending is a change being weighed: the transaction it runs in, the
workspace as locked and the request.
*/
type pending struct {
	tx        *database.Tx
	workspace models.Workspace
	in        UpdateAccessPolicyInput
}

/*
UpdateAccessPolicy changes a workspace's rules.

Rules:
  - The caller needs security_policy:manage.
  - A way of signing in must be one the install offers, and a list must
    keep the caller's own way and one that an active owner can use.
  - A Google domain list must keep the caller's domain when they signed
    in through Google.
  - Session limits may not exceed what the install allows.
  - A dashboard allowlist must keep the caller's address; an API
    allowlist binds no browser and is not weighed against the caller.
  - Values equal to the current ones change nothing.

Events: [appevents.WorkspaceAccessPolicyChanged] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSignInMethodExcludesCaller],
[exceptions.ErrNoOwnerCanSignIn], [exceptions.ErrIPAllowlistExcludesCaller],
a [validation.Error] for values out of range, or a wrapped database error.
*/
type UpdateAccessPolicy struct {
	DB        *database.DB
	Events    *events.Dispatcher
	Mail      *mail.Manager
	Links     appauth.Links
	Show      *ShowAccessPolicy
	MaxAge    time.Duration
	IdleLimit time.Duration
}

/*
Handle applies the change and returns the rules as they now are.
*/
func (a *UpdateAccessPolicy) Handle(ctx context.Context, in UpdateAccessPolicyInput) (Policy, error) {
	if !in.Membership.Can(managePolicy) {
		return Policy{}, exceptions.ErrForbidden
	}

	var policy Policy

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := q.LockWorkspace(ctx, in.Membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		updated, err := a.apply(ctx, pending{tx: tx, workspace: current, in: in})
		if err != nil {
			return err
		}

		policy, err = a.Show.policyOf(ctx, tx, in.Principal, updated)

		return err
	})
	if err != nil {
		return Policy{}, err
	}

	return policy, nil
}

/*
apply checks the change against the workspace, stores it and announces
it, and returns the workspace as it now is.
*/
func (a *UpdateAccessPolicy) apply(ctx context.Context, p pending) (models.Workspace, error) {
	before := p.workspace.Security()
	after, keys, err := a.merge(ctx, before, p.in.Change)
	if err != nil {
		return models.Workspace{}, err
	}

	if err := guard(ctx, p, after); err != nil {
		return models.Workspace{}, err
	}

	changes := diff(before, after, keys)
	if len(changes) == 0 {
		return p.workspace, nil
	}

	settings, err := models.WithAccessPolicy(p.workspace.Settings, keys)
	if err != nil {
		return models.Workspace{}, err
	}

	return a.store(ctx, p, settings, changes)
}

/*
store writes the settings, tells the other owners and announces the
change.
*/
func (a *UpdateAccessPolicy) store(
	ctx context.Context, p pending, settings json.RawMessage, changes map[string]any,
) (models.Workspace, error) {
	tx, w, actor := p.tx, p.workspace, p.in.Membership.Member.ID

	updated, err := models.New(tx).UpdateWorkspace(ctx, models.UpdateWorkspaceParams{
		ID: w.ID, Name: w.Name, Slug: w.Slug, Timezone: w.Timezone, Locale: w.Locale, Settings: settings,
		Now: a.Show.Clock.Now(),
	})
	if err != nil {
		return models.Workspace{}, fmt.Errorf("update workspace rules: %w", err)
	}

	if err := a.mailOwners(ctx, tx, updated, actor); err != nil {
		return models.Workspace{}, err
	}

	err = a.Events.Dispatch(ctx, tx, appevents.WorkspaceAccessPolicyChanged{
		WorkspaceID: updated.ID, ChangedBy: actor, Changes: changes,
	})

	return updated, err
}

/*
mailOwners tells every active owner but the one who made the change.
*/
func (a *UpdateAccessPolicy) mailOwners(
	ctx context.Context, tx *database.Tx, w models.Workspace, actor ids.UUID,
) error {
	owners, err := models.New(tx).ListActiveOwners(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("list owners to tell: %w", err)
	}

	for _, owner := range owners {
		if owner.WorkspaceMember.ID == actor {
			continue
		}

		err := appmail.ToUser(a.Mail, owner.User).QueueTx(ctx, tx, appmail.AccessPolicyChangedMail{
			Name: owner.User.Name, Workspace: w.Name, SettingsURL: a.Links.WorkspaceSecurity(),
		})
		if err != nil {
			return fmt.Errorf("queue access policy mail: %w", err)
		}
	}

	return nil
}

/*
merge validates the change, returning the rules after it and the stored
value of each setting it touches.
*/
func (a *UpdateAccessPolicy) merge(
	ctx context.Context, settings models.SecuritySettings, c Change,
) (models.SecuritySettings, models.AccessPolicyChange, error) {
	keys := models.AccessPolicyChange{}

	if c.AllowedSignInMethods != nil {
		methods := compact(*c.AllowedSignInMethods)
		if !allOf(methods, a.Show.available(ctx, settings)) {
			return settings, nil, violation("allowed_sign_in_methods", "in")
		}

		settings.AllowedSignInMethods, keys["allowed_sign_in_methods"] = methods, methods
	}

	if c.GoogleAllowedDomains != nil {
		domains := compact(*c.GoogleAllowedDomains)
		settings.GoogleAllowedDomains, keys["google_allowed_domains"] = domains, domains
	}

	if err := a.mergeLimits(&settings, keys, c); err != nil {
		return settings, nil, err
	}

	if c.DashboardAllowlist != nil {
		list, err := cleanList(*c.DashboardAllowlist, "dashboard_ip_allowlist")
		if err != nil {
			return settings, nil, err
		}

		settings.DashboardAllowlist, keys["dashboard_ip_allowlist"] = list, list
	}

	if c.APIAllowlist != nil {
		list, err := cleanList(*c.APIAllowlist, "api_ip_allowlist")
		if err != nil {
			return settings, nil, err
		}

		settings.APIAllowlist, keys["api_ip_allowlist"] = list, list
	}

	return settings, keys, nil
}

/*
mergeLimits applies the session limits, refusing one longer than the
install itself allows.
*/
func (a *UpdateAccessPolicy) mergeLimits(
	settings *models.SecuritySettings, keys models.AccessPolicyChange, c Change,
) error {
	switch {
	case c.ClearSessionMaxAge:
		settings.SessionMaxAge, keys["session_max_age_minutes"] = nil, nil
	case c.SessionMaxAge != nil:
		if time.Duration(*c.SessionMaxAge)*time.Minute > a.MaxAge {
			return violation("session_max_age_minutes", "max")
		}

		settings.SessionMaxAge, keys["session_max_age_minutes"] = c.SessionMaxAge, *c.SessionMaxAge
	}

	switch {
	case c.ClearSessionIdleTimeout:
		settings.SessionIdleTimeout, keys["session_idle_timeout_minutes"] = nil, nil
	case c.SessionIdleTimeout != nil:
		if time.Duration(*c.SessionIdleTimeout)*time.Minute > a.IdleLimit {
			return violation("session_idle_timeout_minutes", "max")
		}

		settings.SessionIdleTimeout, keys["session_idle_timeout_minutes"] = c.SessionIdleTimeout, *c.SessionIdleTimeout
	}

	return nil
}

/*
guard refuses rules that would shut out the caller or every owner. It
only weighs what the change touches, so a change to one rule is never
refused over another.
*/
func guard(ctx context.Context, pend pending, after models.SecuritySettings) error {
	p, c := pend.in.Principal, pend.in.Change

	if c.AllowedSignInMethods != nil {
		if err := guardMethods(ctx, pend, after.AllowedSignInMethods); err != nil {
			return err
		}
	}

	if c.GoogleAllowedDomains != nil && len(after.GoogleAllowedDomains) > 0 &&
		p.SignIn.Method == appauth.MethodGoogle && !slices.Contains(after.GoogleAllowedDomains, p.AuthDomain) {
		return exceptions.ErrSignInMethodExcludesCaller.OnField("google_allowed_domains")
	}

	if c.DashboardAllowlist != nil && !models.AddressAllowed(after.DashboardAllowlist, p.IP) {
		return exceptions.ErrIPAllowlistExcludesCaller.With("ip", address(p))
	}

	return nil
}

/*
address is the caller's address as text, "" when it could not be told.
*/
func address(p *appauth.Principal) string {
	if !p.IP.IsValid() {
		return ""
	}

	return p.IP.String()
}

/*
guardMethods refuses a list that leaves out how the caller signed in or
that no active owner could use.
*/
func guardMethods(ctx context.Context, p pending, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}

	if !appauth.SignInAllowed(p.in.Principal.SignIn.Method, allowed) {
		return exceptions.ErrSignInMethodExcludesCaller
	}

	q := models.New(p.tx)

	owners, err := q.ListActiveOwners(ctx, p.workspace.ID)
	if err != nil {
		return fmt.Errorf("list owners to weigh: %w", err)
	}

	userIDs := make([]ids.UUID, 0, len(owners))
	for _, owner := range owners {
		userIDs = append(userIDs, owner.User.ID)
	}

	linked, err := q.ListUserIdentityProviders(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("list owners' providers: %w", err)
	}

	for _, owner := range owners {
		if ownerCanSignIn(owner.User, linked, allowed) {
			return nil
		}
	}

	return exceptions.ErrNoOwnerCanSignIn
}

/*
ownerCanSignIn reports whether the owner holds one of the allowed ways
in: a password, a proven mailbox for a magic link, or a linked provider
or single sign-on identity.
*/
func ownerCanSignIn(u models.User, linked []models.ListUserIdentityProvidersRow, allowed []string) bool {
	for _, method := range allowed {
		switch method {
		case appauth.MethodPassword:
			if u.HasPassword() {
				return true
			}
		case appauth.MethodMagicLink:
			if u.IsVerified() {
				return true
			}
		default:
			if slices.ContainsFunc(linked, func(row models.ListUserIdentityProvidersRow) bool {
				return row.UserID == u.ID && row.Provider == method
			}) {
				return true
			}
		}
	}

	return false
}

/*
cleanList normalizes an allowlist, refusing an entry that does not parse.
*/
func cleanList(list []models.AllowedAddress, field string) ([]models.AllowedAddress, error) {
	out := make([]models.AllowedAddress, 0, len(list))

	for _, entry := range list {
		prefix, err := models.ParseAllowedAddress(strings.TrimSpace(entry.CIDR))
		if err != nil {
			return nil, violation(field, "cidr")
		}

		out = append(out, models.AllowedAddress{CIDR: prefix.String(), Label: str.Clean(entry.Label)})
	}

	return out, nil
}

/*
diff returns, per setting the change touched, what it was and what it
became, leaving out those that stayed the same.
*/
func diff(before, after models.SecuritySettings, keys models.AccessPolicyChange) map[string]any {
	was, now := asMap(before), asMap(after)
	changes := map[string]any{}

	for key := range keys {
		if !same(was[key], now[key]) {
			changes[key] = map[string]any{"from": was[key], "to": now[key]}
		}
	}

	return changes
}

/*
same reports whether two decoded values encode alike.
*/
func same(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)

	return bytes.Equal(left, right)
}

/*
asMap returns the rules by their stored names, with empty lists and
absent lists reading the same.
*/
func asMap(s models.SecuritySettings) map[string]any {
	raw, _ := json.Marshal(s)
	values := map[string]any{}
	_ = json.Unmarshal(raw, &values)

	for key, value := range values {
		if list, ok := value.([]any); ok && len(list) == 0 {
			values[key] = nil
		}
	}

	return values
}

/*
compact returns the values trimmed, lowercased and without repeats.
*/
func compact(values []string) []string {
	out := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}

	return out
}

/*
allOf reports whether every value is one of those offered.
*/
func allOf(values, offered []string) bool {
	return !slices.ContainsFunc(values, func(value string) bool { return !slices.Contains(offered, value) })
}

/*
violation is the refusal of one field's value, named the way validation
errors are.
*/
func violation(field, rule string) error {
	return &validation.Error{Violations: []validation.Violation{{
		Field: field, Rule: rule, Message: "The value of " + field + " is not allowed.",
	}}}
}
