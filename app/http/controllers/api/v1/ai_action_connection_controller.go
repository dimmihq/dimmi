/*
|--------------------------------------------------------------------------
| Agent Action Connection Controller
|--------------------------------------------------------------------------
|
| The connections the workspace's actions reach: listed, created,
| changed, deleted, probed, and for an MCP server, synced into draft
| actions. Secrets come in and never go out; every id is a public one.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIActionConnectionController serves the action connections.
*/
type AIActionConnectionController struct {
	Connections *agentactions.Connections
	IDs         *ids.Registry
}

/*
AIActionConnectionsOutput is the connection list.
*/
type AIActionConnectionsOutput struct {
	Body fhttp.Collection[resources.AIActionConnection]
}

/*
AIActionConnectionOutput is one connection.
*/
type AIActionConnectionOutput struct {
	Body fhttp.Resource[resources.AIActionConnection]
}

/*
AIActionConnectionTestOutput is a probe's result.
*/
type AIActionConnectionTestOutput struct {
	Body fhttp.Resource[resources.AIActionConnectionTest]
}

/*
AIActionToolSyncOutput is a tool sync's result.
*/
type AIActionToolSyncOutput struct {
	Body fhttp.Resource[resources.AIActionToolSync]
}

/*
Index lists the connections.
*/
func (c *AIActionConnectionController) Index(ctx context.Context, _ *requests.AIAgentRequest,
) (*AIActionConnectionsOutput, error) {
	rows, err := c.Connections.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	counts, err := c.Connections.ActionCounts(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	items := resources.NewAIActionConnections(rows, counts)
	page := pagination.Page[resources.AIActionConnection]{Items: items, Limit: 100}
	return &AIActionConnectionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store creates a connection.
*/
func (c *AIActionConnectionController) Store(ctx context.Context, in *requests.AIStoreActionConnectionRequest,
) (*AIActionConnectionOutput, error) {
	row, err := c.Connections.Create(ctx, connectionInput(ctx, in.Body))
	if err != nil {
		return nil, err
	}
	return &AIActionConnectionOutput{Body: fhttp.NewResource(resources.NewAIActionConnection(row))}, nil
}

/*
Show answers one connection.
*/
func (c *AIActionConnectionController) Show(ctx context.Context, in *requests.AIActionConnectionRequest,
) (*AIActionConnectionOutput, error) {
	id, err := c.id(in.Connection)
	if err != nil {
		return nil, err
	}
	row, err := c.Connections.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return c.output(ctx, row)
}

/*
output answers one connection with how many actions use it.
*/
func (c *AIActionConnectionController) output(ctx context.Context, row models.AiActionConnection,
) (*AIActionConnectionOutput, error) {
	counts, err := c.Connections.ActionCounts(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	out := resources.NewAIActionConnections([]models.AiActionConnection{row}, counts)[0]
	return &AIActionConnectionOutput{Body: fhttp.NewResource(out)}, nil
}

/*
Update changes a connection.
*/
func (c *AIActionConnectionController) Update(ctx context.Context, in *requests.AIUpdateActionConnectionRequest,
) (*AIActionConnectionOutput, error) {
	id, err := c.id(in.Connection)
	if err != nil {
		return nil, err
	}
	row, err := c.Connections.Update(ctx, id, connectionInput(ctx, in.Body))
	if err != nil {
		return nil, err
	}
	return c.output(ctx, row)
}

/*
Destroy deletes a connection.
*/
func (c *AIActionConnectionController) Destroy(ctx context.Context, in *requests.AIDestroyActionConnectionRequest,
) (*EmptyOutput, error) {
	id, err := c.id(in.Connection)
	if err != nil {
		return nil, err
	}
	return &EmptyOutput{}, c.Connections.Delete(ctx, agentactions.Removal{
		Membership: membership(ctx), ConnectionID: id, Force: in.Force,
	})
}

/*
Test probes a connection.
*/
func (c *AIActionConnectionController) Test(ctx context.Context, in *requests.AITestActionConnectionRequest,
) (*AIActionConnectionTestOutput, error) {
	id, err := c.id(in.Connection)
	if err != nil {
		return nil, err
	}
	conn, err := c.Connections.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	res, err := c.Connections.Test(ctx, membership(ctx), id, in.Body.Path)
	if err != nil {
		return nil, err
	}
	return &AIActionConnectionTestOutput{
		Body: fhttp.NewResource(resources.NewAIActionConnectionTest(conn.Kind, res)),
	}, nil
}

/*
SyncTools turns an MCP server's tools into draft actions.
*/
func (c *AIActionConnectionController) SyncTools(ctx context.Context, in *requests.AIActionConnectionRequest,
) (*AIActionToolSyncOutput, error) {
	id, err := c.id(in.Connection)
	if err != nil {
		return nil, err
	}
	res, err := c.Connections.SyncTools(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &AIActionToolSyncOutput{Body: fhttp.NewResource(resources.NewAIActionToolSync(res))}, nil
}

/*
id decodes a connection's public id; an unknown one is not found.
*/
func (c *AIActionConnectionController) id(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAiActionConnection), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}

/*
connectionInput maps a request body to the action's input.
*/
func connectionInput(ctx context.Context, b requests.AIActionConnectionBody) agentactions.ConnectionInput {
	return agentactions.ConnectionInput{
		Membership: membership(ctx), Name: b.Name, Kind: b.Kind, BaseURL: b.BaseURL, AuthType: b.AuthType,
		AuthConfig: b.AuthConfig, Token: b.Token, Username: b.Username, Password: b.Password,
		DefaultHeaders: b.DefaultHeaders, Variables: b.Variables, TimeoutMS: b.TimeoutMS, Status: b.Status,
	}
}
