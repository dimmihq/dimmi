/*
|--------------------------------------------------------------------------
| Help Centers
|--------------------------------------------------------------------------
|
| A workspace's public docs live in help centers, one of them the
| default. A workspace gets its default the first time anyone looks,
| named after the workspace and reachable at its slug, so docs work
| before anyone opens the settings. A subdomain is unique across the
| install and a few are kept for the install itself. Every change a
| reader could see moves the help center's cache version on, which is
| how every replica's page cache knows to let go.
|
*/

package helpcenters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing help centers.
*/
var (
	ErrHelpCenterNotFound = fault.New(fault.NotFound, "help_center_not_found",
		"No help center of this workspace has this id.")
	ErrSubdomainTaken = fault.New(fault.Conflict, "subdomain_taken",
		"Another help center already uses this address.").OnField("subdomain")
	ErrSubdomainInvalid = fault.New(fault.Invalid, "subdomain_invalid",
		"Use 3 to 40 lowercase letters, digits and dashes, starting and ending with a letter or digit.").
		OnField("subdomain")
	ErrSubdomainReserved = fault.New(fault.Invalid, "subdomain_reserved",
		"That address is kept for dimmi itself; pick another.").OnField("subdomain")
)

/*
HelpCenterSettings is how a help center looks and what its home page says.
LoginURL is where readers sign in to the team's product, which sends them
back signed in, so they can read what is kept to an audience.
*/
type HelpCenterSettings struct {
	PrimaryColor string `json:"primary_color,omitempty"`
	Appearance   string `json:"appearance,omitempty"`
	HomeTitle    string `json:"home_title,omitempty"`
	HomeSubtitle string `json:"home_subtitle,omitempty"`
	FooterText   string `json:"footer_text,omitempty"`
	ContactURL   string `json:"contact_url,omitempty"`
	LoginURL     string `json:"login_url,omitempty"`
}

/*
SettingsOf reads a help center's settings.
*/
func SettingsOf(h models.HelpCenter) HelpCenterSettings {
	var s HelpCenterSettings
	_ = json.Unmarshal(h.Settings, &s)
	return s
}

/*
HelpCenters manages help centers.
*/
type HelpCenters struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
reserved are subdomains kept for the install itself.
*/
func reserved() []string {
	return []string{
		"www", "app", "api", "admin", "mail", "status", "docs", "help", "changelog", "feedback", "dimmi", "support",
		"blog", "cdn", "assets", "static", "auth", "login", "account", "billing", "security", "dashboard",
	}
}

/*
Name returns "default_help_center".
*/
func (h *HelpCenters) Name() string { return "default_help_center" }

/*
Initialize gives a workspace without a help center its default one and
points its messengers without a help center at it.
*/
func (h *HelpCenters) Initialize(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	return h.ensure(ctx, models.New(tx), w)
}

/*
ensure makes the workspace's default help center when there is none.
*/
func (h *HelpCenters) ensure(ctx context.Context, q *models.Queries, w models.Workspace) error {
	_, err := q.FindDefaultHelpCenter(ctx, w.ID)
	if err == nil || !errors.Is(err, database.ErrNotFound) {
		return err
	}
	sub, err := h.freeSubdomain(ctx, q, w.Slug)
	if err != nil {
		return err
	}
	now := h.Clock.Now()
	created, err := q.CreateHelpCenter(ctx, models.CreateHelpCenterParams{
		ID: ids.NewV7At(now), WorkspaceID: w.ID, Name: truncate(w.Name+" Help", 80), Subdomain: sub,
		Locale: localeOf(w), IsDefault: true, Settings: json.RawMessage(`{}`), Now: now,
	})
	if err != nil {
		return fmt.Errorf("create default help center: %w", err)
	}
	if err := q.PointWidgetsAtHelpCenter(ctx, models.PointWidgetsAtHelpCenterParams{
		WorkspaceID: w.ID, HelpCenterID: &created.ID, Now: now,
	}); err != nil {
		return fmt.Errorf("point widgets at help center: %w", err)
	}
	return nil
}

/*
freeSubdomain returns the slug as a subdomain, or with a short suffix
when it is taken or kept.
*/
func (h *HelpCenters) freeSubdomain(ctx context.Context, q *models.Queries, slug string) (string, error) {
	base := Slugify(slug, 30)
	if len(base) < 3 {
		base = "help-" + base
	}
	candidate := base
	for range 5 {
		if !slices.Contains(reserved(), candidate) {
			taken, err := q.HelpCenterSubdomainTaken(ctx, candidate)
			if err != nil {
				return "", fmt.Errorf("check subdomain: %w", err)
			}
			if !taken {
				return candidate, nil
			}
		}
		candidate = strings.Trim(base, "-") + "-" + strings.ToLower(str.Random(4))
	}
	return "", ErrSubdomainTaken
}

/*
List returns the workspace's help centers, making the default one when
there is none yet.
*/
func (h *HelpCenters) List(ctx context.Context, m authz.Membership) ([]models.HelpCenter, error) {
	if !m.Can("articles:read") && !m.Can("help_centers:read_settings") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.HelpCenter
	err := h.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if err := h.ensure(ctx, q, m.Workspace); err != nil {
			return err
		}
		var err error
		out, err = q.ListHelpCenters(ctx, m.Workspace.ID)
		return err
	})
	return out, err
}

/*
Show returns one help center.
*/
func (h *HelpCenters) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.HelpCenter, error) {
	if !m.Can("articles:read") && !m.Can("help_centers:read_settings") {
		return models.HelpCenter{}, exceptions.ErrForbidden
	}
	var out models.HelpCenter
	err := h.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findHelpCenter(ctx, models.New(tx), m.Workspace.ID, id)
		return err
	})
	return out, err
}

/*
HelpCenterChange is a change to a help center; nil leaves a field as it
is.
*/
type HelpCenterChange struct {
	Membership authz.Membership
	ID         ids.UUID
	Name       *string
	Subdomain  *string
	Locale     *string
	Status     *string
	Indexing   *bool
	AskAI      *bool
	Settings   *HelpCenterSettings
}

/*
Update changes a help center.
*/
func (h *HelpCenters) Update(ctx context.Context, in HelpCenterChange) (models.HelpCenter, error) {
	if !in.Membership.Can("help_centers:manage") {
		return models.HelpCenter{}, exceptions.ErrForbidden
	}
	var out models.HelpCenter
	err := h.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findHelpCenter(ctx, q, in.Membership.Workspace.ID, in.ID)
		if err != nil {
			return err
		}
		params, err := h.apply(ctx, q, current, in)
		if err != nil {
			return err
		}
		out, err = q.UpdateHelpCenter(ctx, params)
		if err != nil {
			return fmt.Errorf("update help center: %w", err)
		}
		return nil
	})
	return out, err
}

/*
apply checks the change and writes it over the current help center.
*/
func (h *HelpCenters) apply(ctx context.Context, q *models.Queries, c models.HelpCenter, in HelpCenterChange,
) (models.UpdateHelpCenterParams, error) {
	p := models.UpdateHelpCenterParams{
		Name: c.Name, Subdomain: c.Subdomain, Locale: c.Locale, Status: c.Status, Settings: c.Settings,
		IndexingEnabled: c.IndexingEnabled, AskAiEnabled: c.AskAiEnabled, Now: h.Clock.Now(),
		WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len([]rune(name)) > 80 {
			return p, fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").OnField("name")
		}
		p.Name = name
	}
	if in.Subdomain != nil && !strings.EqualFold(*in.Subdomain, c.Subdomain) {
		sub, err := checkSubdomain(ctx, q, *in.Subdomain)
		if err != nil {
			return p, err
		}
		p.Subdomain = sub
	}
	if err := applyFlags(&p, in); err != nil {
		return p, err
	}
	if in.Settings != nil {
		raw, err := checkSettings(*in.Settings)
		if err != nil {
			return p, err
		}
		p.Settings = raw
	}
	return p, nil
}

/*
applyFlags writes the change's language, status and switches.
*/
func applyFlags(p *models.UpdateHelpCenterParams, in HelpCenterChange) error {
	if in.Locale != nil {
		p.Locale = *in.Locale
	}
	if in.Status != nil {
		if *in.Status != "active" && *in.Status != "disabled" {
			return fault.New(fault.Invalid, "status_invalid", "The status is active or disabled.").OnField("status")
		}
		p.Status = *in.Status
	}
	if in.Indexing != nil {
		p.IndexingEnabled = *in.Indexing
	}
	if in.AskAI != nil {
		p.AskAiEnabled = *in.AskAI
	}
	return nil
}

/*
checkSubdomain refuses a malformed, kept or taken subdomain.
*/
func checkSubdomain(ctx context.Context, q *models.Queries, raw string) (string, error) {
	sub := strings.ToLower(strings.TrimSpace(raw))
	if len(sub) < 3 || len(sub) > 40 || !slugPattern().MatchString(sub) {
		return "", ErrSubdomainInvalid
	}
	if slices.Contains(reserved(), sub) {
		return "", ErrSubdomainReserved
	}
	taken, err := q.HelpCenterSubdomainTaken(ctx, sub)
	if err != nil {
		return "", fmt.Errorf("check subdomain: %w", err)
	}
	if taken {
		return "", ErrSubdomainTaken
	}
	return sub, nil
}

/*
checkSettings refuses colors, links and texts readers must not get.
*/
func checkSettings(s HelpCenterSettings) (json.RawMessage, error) {
	if s.PrimaryColor != "" && !hexColor(s.PrimaryColor) {
		return nil, fault.New(fault.Invalid, "primary_color_invalid", "Use a hex color such as #10b981.").
			OnField("settings.primary_color")
	}
	if s.Appearance != "" && !slices.Contains([]string{"auto", "light", "dark"}, s.Appearance) {
		return nil, fault.New(fault.Invalid, "appearance_invalid", "The appearance is auto, light or dark.").
			OnField("settings.appearance")
	}
	if s.ContactURL != "" && !strings.HasPrefix(s.ContactURL, "https://") {
		return nil, fault.New(fault.Invalid, "contact_url_invalid", "Use an https address.").
			OnField("settings.contact_url")
	}
	if s.LoginURL != "" && !secureURL(s.LoginURL) {
		return nil, fault.New(fault.Invalid, "login_url_invalid", "Use an https address.").
			OnField("settings.login_url")
	}
	s.HomeTitle, s.HomeSubtitle = truncate(s.HomeTitle, 80), truncate(s.HomeSubtitle, 200)
	s.FooterText = truncate(s.FooterText, 500)
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("encode help center settings: %w", err)
	}
	return raw, nil
}

func hexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

/*
findHelpCenter returns a live help center of the workspace, or 404.
*/
func findHelpCenter(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.HelpCenter, error) {
	out, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrHelpCenterNotFound
	}
	if err != nil {
		return out, fmt.Errorf("find help center: %w", err)
	}
	return out, nil
}

func localeOf(w models.Workspace) string {
	if w.Locale == "" {
		return "en"
	}
	return w.Locale
}

func truncate(s string, limit int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > limit {
		return string(r[:limit])
	}
	return s
}

/*
secureURL accepts an https address, or plain http on this machine for
development.
*/
func secureURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return false
	}

	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))
}
