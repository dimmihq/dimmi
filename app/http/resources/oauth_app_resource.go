/*
|--------------------------------------------------------------------------
| Connected App Resources
|--------------------------------------------------------------------------
|
| What the consent screen shows of an app's request, where the browser
| goes once the person decided, and the apps a person connected.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/oauthapps"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OAuthAuthorization is an app's request as the person reviews it. SelfMade
says the app registered itself, so dimmi cannot vouch for it, and
Loopback that it answers on the person's own computer.
*/
type OAuthAuthorization struct {
	Object       string                 `json:"object"`
	ClientID     string                 `json:"client_id"`
	ClientName   string                 `json:"client_name"`
	SelfMade     bool                   `json:"self_registered"`
	RedirectHost string                 `json:"redirect_host"`
	Loopback     bool                   `json:"loopback"`
	Scopes       []string               `json:"scopes"`
	Workspaces   []OAuthWorkspaceOption `json:"workspaces"`
}

/*
OAuthWorkspaceOption is a workspace the person may connect the app to.
*/
type OAuthWorkspaceOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

/*
NewOAuthAuthorization shapes a reviewed request.
*/
func NewOAuthAuthorization(r oauthapps.Request) OAuthAuthorization {
	out := OAuthAuthorization{
		Object: "oauth_authorization", ClientID: r.ClientID, ClientName: r.ClientName, SelfMade: r.SelfMade,
		RedirectHost: r.RedirectHost, Loopback: r.Loopback, Scopes: r.Scopes,
		Workspaces: make([]OAuthWorkspaceOption, len(r.Workspaces)),
	}
	for i, w := range r.Workspaces {
		out.Workspaces[i] = OAuthWorkspaceOption{
			ID: models.PublicID(models.PrefixWorkspace, w.WorkspaceID), Name: w.WorkspaceName, Role: w.Role,
		}
	}

	return out
}

/*
OAuthDecision is where the browser goes once the person decided.
*/
type OAuthDecision struct {
	Object     string `json:"object"`
	RedirectTo string `json:"redirect_to"`
}

/*
ConnectedApp is an app a person connected.
*/
type ConnectedApp struct {
	ID            string    `json:"id"`
	Object        string    `json:"object"`
	ClientID      string    `json:"client_id"`
	Name          string    `json:"name"`
	WorkspaceID   string    `json:"workspace_id,omitempty"`
	WorkspaceName string    `json:"workspace_name"`
	Scopes        []string  `json:"scopes"`
	CreatedAt     time.Time `json:"created_at"`
	LastUsedAt    time.Time `json:"last_used_at"`
}

/*
NewConnectedApps shapes a person's connected apps.
*/
func NewConnectedApps(apps []oauthapps.App) []ConnectedApp {
	out := make([]ConnectedApp, len(apps))
	for i, a := range apps {
		out[i] = ConnectedApp{
			ID: models.PublicID(models.PrefixOAuthGrant, a.ID), Object: "connected_app", ClientID: a.ClientID,
			Name: a.ClientName, WorkspaceName: a.WorkspaceName, Scopes: a.Scopes, CreatedAt: a.CreatedAt,
			LastUsedAt: a.LastUsedAt,
		}
		if a.WorkspaceName != "" {
			out[i].WorkspaceID = models.PublicID(models.PrefixWorkspace, a.WorkspaceID)
		}
	}

	return out
}
