/*
|--------------------------------------------------------------------------
| Changing Link Safety
|--------------------------------------------------------------------------
|
| The allowed and blocked domains are hostnames a workspace could own:
| stored lower case, in their ASCII form, without duplicates. A public
| suffix such as co.uk is refused, since blocking it would block every
| site under it. Each list holds at most 500 domains.
|
*/

package workspaces

import (
	"slices"

	"github.com/dimmihq/dimmi/framework/customdomain"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxLinkDomains is the most domains each list holds.
*/
const MaxLinkDomains = 500

/*
LinkSafetyChange is a change to the link checks; nil leaves a part as
it is.
*/
type LinkSafetyChange struct {
	Enabled                *bool
	AllowDomains           []string
	BlockDomains           []string
	StripInboundEmailLinks *bool
}

/*
applyLinkSafety writes the change into the settings and reports whether
anything changed.
*/
func applyLinkSafety(params *models.UpdateWorkspaceParams, w models.Workspace, change *LinkSafetyChange) (bool, error) {
	if change == nil {
		return false, nil
	}

	current := w.LinkSafety()
	next := current

	if change.Enabled != nil {
		next.Enabled = *change.Enabled
	}

	if change.StripInboundEmailLinks != nil {
		next.StripInboundEmailLinks = *change.StripInboundEmailLinks
	}

	var err error

	if next.AllowDomains, err = domains(change.AllowDomains, current.AllowDomains); err != nil {
		return false, err
	}

	if next.BlockDomains, err = domains(change.BlockDomains, current.BlockDomains); err != nil {
		return false, err
	}

	if sameLinkSafety(current, next) {
		return false, nil
	}

	params.Settings, err = models.WithArea(params.Settings, "link_safety", next)

	return err == nil, err
}

/*
domains returns the list normalized, or the current one when none was
sent.
*/
func domains(sent, current []string) ([]string, error) {
	if sent == nil {
		return current, nil
	}

	if len(sent) > MaxLinkDomains {
		return nil, exceptions.ErrInvalidLinkDomains
	}

	out := make([]string, 0, len(sent))

	for _, domain := range sent {
		name, err := customdomain.Normalize(domain)
		if err != nil {
			return nil, exceptions.ErrInvalidLinkDomains.With("domain", domain)
		}

		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}

	return out, nil
}

/*
sameLinkSafety reports whether two settings are the same.
*/
func sameLinkSafety(a, b models.LinkSafety) bool {
	return a.Enabled == b.Enabled && a.StripInboundEmailLinks == b.StripInboundEmailLinks &&
		slices.Equal(a.AllowDomains, b.AllowDomains) && slices.Equal(a.BlockDomains, b.BlockDomains)
}
