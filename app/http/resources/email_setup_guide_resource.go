/*
|--------------------------------------------------------------------------
| Forwarding Setup Guides
|--------------------------------------------------------------------------
|
| Forwarding is set up in the mail service the public address lives in,
| not here, so each address carries the steps for every service people
| use, its own addresses filled in and the one that fits its domain
| first and marked. Provider repeats the key for older readers.
|
*/

package resources

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/integrations/email/setupguides"
)

/*
SetupGuide is how to forward mail to the address in one mail service:
the steps in order, what tends to go wrong there, and a link to the
right settings screen when there is one.
*/
type SetupGuide struct {
	Key         string   `json:"key"`
	Provider    string   `json:"provider"`
	Name        string   `json:"name"`
	Recommended bool     `json:"recommended"`
	DeepLink    *string  `json:"deep_link"`
	Steps       []string `json:"steps"`
	Warnings    []string `json:"warnings"`
}

/*
WithGuides fills in the setup guides, the recommended one first.
*/
func (a EmailAddress) WithGuides(ctx context.Context, recommended string) EmailAddress {
	domain := a.Address[strings.LastIndex(a.Address, "@")+1:]
	args := i18n.Args{"address": a.Address, "forwarding_address": a.ForwardingAddress, "domain": domain}

	guides := setupguides.Ordered(recommended)
	a.SetupGuides = make([]SetupGuide, 0, len(guides))

	for _, g := range guides {
		g.DeepLink = strings.ReplaceAll(g.DeepLink, "{domain}", domain)
		a.SetupGuides = append(a.SetupGuides, newSetupGuide(ctx, g, recommended, args))
	}

	return a
}

func newSetupGuide(ctx context.Context, g setupguides.Guide, recommended string, args i18n.Args) SetupGuide {
	out := SetupGuide{
		Key: g.Key, Provider: g.Key, Name: g.Name, Recommended: g.Key == recommended,
		Steps: make([]string, 0, len(g.Steps)), Warnings: make([]string, 0, len(g.Warnings)),
	}

	if g.DeepLink != "" {
		link := g.DeepLink
		out.DeepLink = &link
	}

	for _, key := range g.Steps {
		out.Steps = append(out.Steps, i18n.T(ctx, key, args))
	}

	for _, key := range g.Warnings {
		out.Warnings = append(out.Warnings, i18n.T(ctx, key, args))
	}

	return out
}
