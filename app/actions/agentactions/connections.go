/*
|--------------------------------------------------------------------------
| Action Connections
|--------------------------------------------------------------------------
|
| Where the agent's actions go: a workspace API or a remote MCP server,
| with its credential. Any member may read connections, never their
| secrets; only those who manage actions change them. A secret is only
| ever written, kept sealed, and shown back as its last four characters.
| A connection that live actions still use cannot be deleted.
|
*/

package agentactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Connections manages a workspace's action connections.
*/
type Connections struct {
	DB    *database.DB
	Clock clock.Clock
	Keys  *encryption.Keys
	HTTP  *httpclient.Client
	Kit   *actionkit.Kit
}

/*
ConnectionInput is a connection to create, or the fields of one to
change: nil leaves a field as it is. Token, Username and Password are
the secret, written only.
*/
type ConnectionInput struct {
	Membership     authz.Membership
	Name           *string
	Kind           string
	BaseURL        *string
	AuthType       *string
	AuthConfig     *actionkit.AuthConfig
	Token          *string
	Username       *string
	Password       *string
	DefaultHeaders *map[string]string
	Variables      *map[string]string
	TimeoutMS      *int32
	Status         *string
}

/*
RequireRead checks the caller may see the workspace's actions.
*/
func RequireRead(m authz.Membership) error {
	if !m.Can("ai_actions:read") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
RequireManage checks the caller may change the workspace's actions.
*/
func RequireManage(m authz.Membership) error {
	if !m.Can("ai_actions:manage") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
List returns the workspace's connections.
*/
func (c *Connections) List(ctx context.Context, m authz.Membership) ([]models.AiActionConnection, error) {
	if err := RequireRead(m); err != nil {
		return nil, err
	}
	var out []models.AiActionConnection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListActionConnections(ctx, m.Workspace.ID)
		return err
	})
	return out, err
}

/*
Show returns one connection.
*/
func (c *Connections) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.AiActionConnection, error) {
	if err := RequireRead(m); err != nil {
		return models.AiActionConnection{}, err
	}
	var out models.AiActionConnection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findConnection(ctx, tx, m.Workspace.ID, id)
		return err
	})
	return out, err
}

/*
Create stores a new connection and seals its secret.
*/
func (c *Connections) Create(ctx context.Context, in ConnectionInput) (models.AiActionConnection, error) {
	if err := RequireManage(in.Membership); err != nil {
		return models.AiActionConnection{}, err
	}
	if in.Kind != "http" && in.Kind != "mcp" {
		return models.AiActionConnection{}, invalidField("kind_invalid", "kind", "kind is http or mcp")
	}
	fields, err := c.fields(models.AiActionConnection{
		Kind: in.Kind, AuthType: actionkit.AuthNone, AuthConfig: json.RawMessage("{}"),
		DefaultHeaders: json.RawMessage("{}"), Variables: json.RawMessage("{}"), TimeoutMs: 8000, Status: "active",
	}, in)
	if err != nil {
		return models.AiActionConnection{}, err
	}

	var out models.AiActionConnection
	err = c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		member := in.Membership.Member.ID
		row, err := models.New(tx).CreateActionConnection(ctx, models.CreateActionConnectionParams{
			WorkspaceID: in.Membership.Workspace.ID, Name: fields.Name, Kind: fields.Kind, BaseUrl: fields.BaseUrl,
			AuthType: fields.AuthType, AuthConfig: fields.AuthConfig, DefaultHeaders: fields.DefaultHeaders,
			Variables: fields.Variables, TimeoutMs: fields.TimeoutMs, CreatedByMemberID: &member, Now: c.Clock.Now(),
		})
		if err != nil {
			return connectionWriteError(err)
		}
		out, err = c.seal(ctx, tx, row, in)
		return err
	})
	return out, err
}

/*
Update changes a connection's fields and, when a new secret is sent,
replaces it.
*/
func (c *Connections) Update(ctx context.Context, id ids.UUID, in ConnectionInput) (models.AiActionConnection, error) {
	if err := RequireManage(in.Membership); err != nil {
		return models.AiActionConnection{}, err
	}
	var out models.AiActionConnection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		current, err := findConnection(ctx, tx, in.Membership.Workspace.ID, id)
		if err != nil {
			return err
		}
		fields, err := c.fields(current, in)
		if err != nil {
			return err
		}
		row, err := models.New(tx).UpdateActionConnection(ctx, models.UpdateActionConnectionParams{
			WorkspaceID: current.WorkspaceID, ID: current.ID, Name: fields.Name, BaseUrl: fields.BaseUrl,
			AuthType: fields.AuthType, AuthConfig: fields.AuthConfig, DefaultHeaders: fields.DefaultHeaders,
			Variables: fields.Variables, TimeoutMs: fields.TimeoutMs, Status: fields.Status,
			StatusReason: fields.StatusReason, Now: c.Clock.Now(),
		})
		if err != nil {
			return connectionWriteError(err)
		}
		out, err = c.seal(ctx, tx, row, in)
		return err
	})
	return out, err
}

/*
fields applies the input to a connection and checks the result.
*/
func (c *Connections) fields(conn models.AiActionConnection, in ConnectionInput) (models.AiActionConnection, error) {
	if err := applyNames(&conn, in, c.HTTP); err != nil {
		return conn, err
	}
	if err := applyAuthFields(&conn, in); err != nil {
		return conn, err
	}
	if in.DefaultHeaders != nil {
		if err := actionkit.CheckHeaders(*in.DefaultHeaders); err != nil {
			return conn, err
		}
		conn.DefaultHeaders, _ = json.Marshal(*in.DefaultHeaders)
	}
	if in.Variables != nil {
		if err := actionkit.CheckVariables(*in.Variables); err != nil {
			return conn, err
		}
		conn.Variables, _ = json.Marshal(*in.Variables)
	}
	if in.TimeoutMS != nil {
		if *in.TimeoutMS < 1000 || *in.TimeoutMS > 15000 {
			return conn, invalidField("timeout_invalid", "timeout_ms", "use 1000 to 15000 milliseconds")
		}
		conn.TimeoutMs = *in.TimeoutMS
	}
	err := applyStatus(&conn, in)
	return conn, err
}

/*
applyNames applies the name and base URL.
*/
func applyNames(conn *models.AiActionConnection, in ConnectionInput, client *httpclient.Client) error {
	if in.Name != nil {
		conn.Name = *in.Name
	}
	if conn.Name == "" || len([]rune(conn.Name)) > 60 {
		return fault.New(fault.Invalid, "name_invalid", "the name needs 1 to 60 characters").OnField("name")
	}
	if in.BaseURL != nil {
		normal, err := actionkit.NormalizeBaseURL(*in.BaseURL, client)
		if err != nil {
			return err
		}
		conn.BaseUrl = normal
	}
	if conn.BaseUrl == "" {
		return fault.New(fault.Invalid, "base_url_required", "a base url is required").OnField("base_url")
	}
	return nil
}

/*
applyAuthFields applies the auth type and its setting.
*/
func applyAuthFields(conn *models.AiActionConnection, in ConnectionInput) error {
	cfg := actionkit.AuthConfig{}
	_ = json.Unmarshal(conn.AuthConfig, &cfg)
	if in.AuthType != nil {
		conn.AuthType = *in.AuthType
	}
	if in.AuthConfig != nil {
		cfg = *in.AuthConfig
	}
	if err := actionkit.CheckAuth(conn.AuthType, cfg); err != nil {
		return err
	}
	conn.AuthConfig, _ = json.Marshal(cfg)
	return nil
}

/*
applyStatus applies an admin's switch between active and disabled.
*/
func applyStatus(conn *models.AiActionConnection, in ConnectionInput) error {
	if in.Status == nil {
		return nil
	}
	switch *in.Status {
	case "active", "disabled":
		conn.Status, conn.StatusReason = *in.Status, nil
		return nil
	}
	return fault.New(fault.Invalid, "status_invalid", "status is active or disabled").OnField("status")
}

/*
seal stores the secret the input sent, if any; switching to no auth
clears it.
*/
func (c *Connections) seal(ctx context.Context, tx *database.Tx, conn models.AiActionConnection,
	in ConnectionInput,
) (models.AiActionConnection, error) {
	creds, sent := secretOf(in)
	if conn.AuthType == actionkit.AuthNone {
		creds, sent = actionkit.Credentials{}, len(conn.CredentialsEncrypted) > 0
	}
	if !sent {
		return conn, nil
	}
	var sealed encryption.Sealed
	if creds != (actionkit.Credentials{}) {
		var err error
		if sealed, err = actionkit.SealCredentials(ctx, c.Keys, conn, creds); err != nil {
			return conn, err
		}
	}
	return models.New(tx).SetActionConnectionCredentials(ctx, models.SetActionConnectionCredentialsParams{
		WorkspaceID: conn.WorkspaceID, ID: conn.ID, CredentialsEncrypted: sealed, SecretHint: creds.Hint(),
		Now: c.Clock.Now(),
	})
}

/*
secretOf reads the secret an input sent.
*/
func secretOf(in ConnectionInput) (actionkit.Credentials, bool) {
	creds := actionkit.Credentials{}
	sent := false
	for _, f := range []struct {
		value *string
		dst   *string
	}{{in.Token, &creds.Token}, {in.Username, &creds.Username}, {in.Password, &creds.Password}} {
		if f.value != nil {
			*f.dst, sent = *f.value, true
		}
	}
	return creds, sent
}

/*
Delete removes a connection no live action uses, shredding its secret.
*/
func (c *Connections) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := RequireManage(m); err != nil {
		return err
	}
	return c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findConnection(ctx, tx, m.Workspace.ID, id); err != nil {
			return err
		}
		used, err := q.CountConnectionActions(ctx, models.CountConnectionActionsParams{
			WorkspaceID: m.Workspace.ID, ConnectionID: id,
		})
		if err != nil {
			return fmt.Errorf("count connection actions: %w", err)
		}
		if used > 0 {
			return exceptions.ErrConnectionInUse
		}
		return q.DeleteActionConnection(ctx, models.DeleteActionConnectionParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: c.Clock.Now(),
		})
	})
}

/*
Test probes a connection and returns how it went.
*/
func (c *Connections) Test(ctx context.Context, m authz.Membership, id ids.UUID, path string,
) (actionkit.ProbeResult, error) {
	conn, err := c.manageable(ctx, m, id)
	if err != nil {
		return actionkit.ProbeResult{}, err
	}
	return c.Kit.Probe(ctx, conn, path)
}

/*
manageable loads a connection the caller may change.
*/
func (c *Connections) manageable(ctx context.Context, m authz.Membership, id ids.UUID,
) (models.AiActionConnection, error) {
	if err := RequireManage(m); err != nil {
		return models.AiActionConnection{}, err
	}
	var conn models.AiActionConnection
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		conn, err = findConnection(ctx, tx, m.Workspace.ID, id)
		return err
	})
	return conn, err
}

/*
findConnection returns a live connection of the workspace, or 404.
*/
func findConnection(ctx context.Context, tx *database.Tx, workspaceID, id ids.UUID,
) (models.AiActionConnection, error) {
	conn, err := models.New(tx).FindActionConnection(ctx, models.FindActionConnectionParams{
		WorkspaceID: workspaceID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return conn, exceptions.ErrNotFound
	}
	if err != nil {
		return conn, fmt.Errorf("find action connection: %w", err)
	}
	return conn, nil
}

/*
connectionWriteError maps a duplicate name to a field error.
*/
func connectionWriteError(err error) error {
	if database.IsUniqueViolation(err, "") {
		return fault.New(fault.Invalid, "name_taken", "another connection already uses this name").OnField("name")
	}
	return fmt.Errorf("save action connection: %w", err)
}
