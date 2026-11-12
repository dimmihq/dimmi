/*
|--------------------------------------------------------------------------
| Integration Resources
|--------------------------------------------------------------------------
|
| Providers and installations as the settings page sees them: what each
| provider's features ask for, and for each installation its health, the
| scopes it holds, the scopes each feature still misses, what uses it and
| the viewer's own grant. A token never appears here.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IntegrationProvider is a provider this install offers. Installable says
its shared app can be installed; otherwise only a custom app's pasted
tokens connect it. Connect says how: oauth, or api_key for a tool that
connects with a key the workspace pastes, which is always installable.
*/
type IntegrationProvider struct {
	Object      string               `json:"object"`
	Name        string               `json:"name"`
	Label       string               `json:"label"`
	Installable bool                 `json:"installable"`
	Connect     string               `json:"connect"`
	Features    []IntegrationFeature `json:"features"`
}

/*
IntegrationFeature is a feature that uses the provider and the scopes it
asks for.
*/
type IntegrationFeature struct {
	Name       string   `json:"name"`
	BotScopes  []string `json:"bot_scopes"`
	UserScopes []string `json:"user_scopes"`
}

/*
NewIntegrationProviders shapes the providers for the API.
*/
func NewIntegrationProviders(views []integrations.ProviderView) []IntegrationProvider {
	out := make([]IntegrationProvider, 0, len(views))

	for _, v := range views {
		p := IntegrationProvider{
			Object: "integration_provider", Name: v.Provider.Name, Label: v.Provider.Label,
			Installable: v.Installable, Connect: "oauth", Features: []IntegrationFeature{},
		}
		if v.Provider.Key {
			p.Connect, p.Installable = "api_key", true
		}

		for _, f := range v.Features {
			p.Features = append(p.Features, IntegrationFeature{
				Name: f.Name, BotScopes: nonNilList(f.Scopes.Bot), UserScopes: nonNilList(f.Scopes.User),
			})
		}

		out = append(out, p)
	}

	return out
}

/*
Integration is one installation.
*/
type Integration struct {
	ID                string                    `json:"id"`
	Object            string                    `json:"object"`
	Provider          string                    `json:"provider"`
	Name              string                    `json:"name"`
	ExternalAccountID *string                   `json:"external_account_id"`
	ExternalAppID     *string                   `json:"external_app_id"`
	Source            string                    `json:"source"`
	Status            string                    `json:"status"`
	Scopes            []string                  `json:"scopes"`
	LastErrorCode     *string                   `json:"last_error_code"`
	LastError         *string                   `json:"last_error"`
	Metadata          map[string]any            `json:"metadata"`
	Features          []IntegrationFeatureState `json:"features"`
	MemberGrantCount  int64                     `json:"member_grant_count"`
	ChannelIDs        []string                  `json:"channel_ids"`
	OwnGrant          *IntegrationMemberGrant   `json:"own_grant"`
	InstalledBy       *string                   `json:"installed_by"`
	InstalledAt       time.Time                 `json:"installed_at"`
	LastCheckedAt     *time.Time                `json:"last_checked_at"`
	DisconnectedAt    *time.Time                `json:"disconnected_at"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

/*
IntegrationFeatureState is a feature on an installation: ready, or
blocked_missing_scope with the scopes it still needs.
*/
type IntegrationFeatureState struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	MissingScopes []string `json:"missing_scopes"`
}

/*
NewIntegration shapes an installation for the API.
*/
func NewIntegration(v integrations.InstallationView) Integration {
	row := v.Installation
	out := Integration{
		ID: models.PublicID(models.PrefixIntegrationInstallation, row.ID), Object: "integration",
		Provider: row.Provider, Name: row.Name, ExternalAccountID: row.ExternalAccountID,
		ExternalAppID: row.ExternalAppID, Source: row.Source, Status: row.Status, Scopes: nonNilList(row.Scopes),
		LastErrorCode: row.LastErrorCode, LastError: row.LastError, Metadata: map[string]any{},
		Features: []IntegrationFeatureState{}, MemberGrantCount: v.Grants, ChannelIDs: []string{},
		InstalledAt: row.InstalledAt, LastCheckedAt: row.LastCheckedAt, DisconnectedAt: row.DisconnectedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	_ = json.Unmarshal(row.Metadata, &out.Metadata)

	if row.InstalledByMemberID != nil {
		member := models.PublicID(models.PrefixMember, *row.InstalledByMemberID)
		out.InstalledBy = &member
	}

	for _, f := range v.Features {
		state := IntegrationFeatureState{Name: f.Name, Status: "ready", MissingScopes: nonNilList(f.Missing)}
		if len(f.Missing) > 0 {
			state.Status = "blocked_missing_scope"
		}

		out.Features = append(out.Features, state)
	}

	for _, id := range v.Consumers {
		out.ChannelIDs = append(out.ChannelIDs, models.PublicID(models.PrefixChannelConnection, id))
	}

	if v.Own != nil {
		grant := NewIntegrationMemberGrant(*v.Own)
		out.OwnGrant = &grant
	}

	return out
}

/*
NewIntegrations shapes a list of installations.
*/
func NewIntegrations(views []integrations.InstallationView) []Integration {
	out := make([]Integration, 0, len(views))
	for _, v := range views {
		out = append(out, NewIntegration(v))
	}

	return out
}

func nonNilList(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
