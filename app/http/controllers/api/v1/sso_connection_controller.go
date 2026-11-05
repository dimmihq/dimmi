/*
|--------------------------------------------------------------------------
| SSO Connection Controller
|--------------------------------------------------------------------------
|
| GET /sso-connections lists the connections of the organization behind
| the workspace in play and GET /sso-connections/{connection} shows one.
| POST creates a draft, PATCH changes settings, status and enforcement,
| and DELETE removes one. Ids of other organizations' connections are
| simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SSOConnectionsOutput is every connection of the organization.
*/
type SSOConnectionsOutput struct {
	Body fhttp.Collection[resources.SSOConnection]
}

/*
SSOConnectionOutput is one connection.
*/
type SSOConnectionOutput struct {
	Body fhttp.Resource[resources.SSOConnection]
}

/*
SSOConnectionController serves the connections of the organization
behind the workspace in play.
*/
type SSOConnectionController struct {
	List   *sso.ListConnections
	Show   *sso.ShowConnection
	Create *sso.CreateConnection
	Update *sso.UpdateConnection
	Delete *sso.DeleteConnection
	IDs    *ids.Registry
	Links  appauth.Links
}

/*
Index lists the connections.
*/
func (c *SSOConnectionController) Index(ctx context.Context, _ *requests.NoInput) (*SSOConnectionsOutput, error) {
	list, err := c.List.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.SSOConnection, 0, len(list))
	for _, conn := range list {
		items = append(items, resources.NewSSOConnection(conn, c.Links))
	}

	page := pagination.Page[resources.SSOConnection]{Items: items, Limit: len(items)}

	return &SSOConnectionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
ShowConnection answers with one connection.
*/
func (c *SSOConnectionController) ShowConnection(
	ctx context.Context, in *requests.ShowSSOConnectionRequest,
) (*SSOConnectionOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixSSOConnection, in.Connection)
	if err != nil {
		return nil, err
	}

	conn, err := c.Show.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return c.output(conn), nil
}

/*
Store creates a draft connection and answers with it.
*/
func (c *SSOConnectionController) Store(
	ctx context.Context, in *requests.StoreSSOConnectionRequest,
) (*SSOConnectionOutput, error) {
	conn, err := c.Create.Handle(ctx, sso.CreateConnectionInput{
		Principal: caller(ctx), Membership: membership(ctx), Protocol: in.Body.Protocol, Name: in.Body.Name,
		Config: configOf(in.Body.Config), ClientSecret: in.Body.ClientSecret,
	})
	if err != nil {
		return nil, err
	}

	return c.output(conn), nil
}

/*
UpdateConnection changes a connection and answers with it as it now is.
*/
func (c *SSOConnectionController) UpdateConnection(
	ctx context.Context, in *requests.UpdateSSOConnectionRequest,
) (*SSOConnectionOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixSSOConnection, in.Connection)
	if err != nil {
		return nil, err
	}

	change, err := c.change(in)
	if err != nil {
		return nil, err
	}

	conn, err := c.Update.Handle(ctx, sso.UpdateConnectionInput{
		Principal: caller(ctx), Membership: membership(ctx), ConnectionID: id, Change: change,
	})
	if err != nil {
		return nil, err
	}

	return c.output(conn), nil
}

/*
Destroy deletes a connection.
*/
func (c *SSOConnectionController) Destroy(
	ctx context.Context, in *requests.ShowSSOConnectionRequest,
) (*EmptyOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixSSOConnection, in.Connection)
	if err != nil {
		return nil, err
	}

	if err := c.Delete.Handle(ctx, membership(ctx), id); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
customRole decodes the custom role new members start with, when the
request sent one; null clears it.
*/
func (c *SSOConnectionController) customRole(
	in *requests.UpdateSSOConnectionRequest, change *sso.ConnectionChange,
) error {
	if !requests.Sent(in.RawBody)["default_custom_role_id"] {
		return nil
	}

	var role *ids.UUID
	if in.Body.DefaultCustom != nil {
		id, err := roleID(c.IDs, *in.Body.DefaultCustom)
		if err != nil {
			return err
		}

		role = &id
	}

	change.DefaultCustomRole = &role

	return nil
}

/*
change turns the request into the action's change, decoding the ids it
names.
*/
func (c *SSOConnectionController) change(in *requests.UpdateSSOConnectionRequest) (sso.ConnectionChange, error) {
	b := in.Body
	change := sso.ConnectionChange{
		Name: b.Name, ClientSecret: b.ClientSecret, Status: b.Status, Enforced: b.Enforced, JITEnabled: b.JITEnabled,
		DefaultRole: b.DefaultRole, ConfirmOwnersCanSignIn: b.ConfirmOwnersCanSignIn,
	}

	if b.Config != nil {
		cfg := configOf(*b.Config)
		change.Config = &cfg
	}

	if err := c.customRole(in, &change); err != nil {
		return change, err
	}

	if b.DefaultTeamIDs != nil {
		teams := make([]ids.UUID, 0, len(*b.DefaultTeamIDs))
		for _, named := range *b.DefaultTeamIDs {
			id, err := decodeID(c.IDs, models.PrefixTeam, named)
			if err != nil {
				return change, err
			}

			teams = append(teams, id)
		}

		change.DefaultTeamIDs = &teams
	}

	if b.GroupRoleMap != nil {
		groups, err := c.groups(*b.GroupRoleMap)
		if err != nil {
			return change, err
		}

		change.GroupRoleMap = &groups
	}

	return change, nil
}

/*
groups decodes the group roles of a request.
*/
func (c *SSOConnectionController) groups(in []requests.SSOGroupRole) ([]sso.GroupRole, error) {
	out := make([]sso.GroupRole, 0, len(in))

	for _, g := range in {
		role := sso.GroupRole{Group: g.Group, Role: g.Role}

		if g.CustomRoleID != nil {
			id, err := roleID(c.IDs, *g.CustomRoleID)
			if err != nil {
				return nil, err
			}

			role.CustomRoleID = &id
		}

		out = append(out, role)
	}

	return out, nil
}

/*
output shapes one connection.
*/
func (c *SSOConnectionController) output(conn sso.Connection) *SSOConnectionOutput {
	return &SSOConnectionOutput{Body: fhttp.NewResource(resources.NewSSOConnection(conn, c.Links))}
}

/*
configOf turns a request's settings into the action's.
*/
func configOf(in requests.SSOConfig) sso.Config {
	return sso.Config{
		IDPEntityID: in.IDPEntityID, IDPSSOURL: in.IDPSSOURL, IDPCertificates: in.IDPCertificates,
		IDPMetadataURL: in.IDPMetadataURL, NameIDFormat: in.NameIDFormat, Issuer: in.Issuer, ClientID: in.ClientID,
		Scopes: in.Scopes, AttributeMap: sso.AttributeMap(in.AttributeMap),
	}
}

/*
decodeID returns the row a path or body names by public id, refusing an
id that names nothing as not found.
*/
func decodeID(registry *ids.Registry, prefix models.Prefix, named string) (ids.UUID, error) {
	id, err := registry.Decode(string(prefix), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}
