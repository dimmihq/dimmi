/*
|--------------------------------------------------------------------------
| Managing Pages And Instagram Accounts
|--------------------------------------------------------------------------
|
| Connecting the account an admin picked from a sign-in, and what a
| workspace does with a connection afterwards: list it, move where its
| new conversations land, and disconnect it, which tells Meta to stop
| delivering here and drops its tokens while its conversations stay.
|
*/

package channels

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConnectSocialInput picks an account of a sign-in: the grant, the
account, and the inbox its conversations land in, the default one when
nil.
*/
type ConnectSocialInput struct {
	Membership authz.Membership
	Grant      string
	AccountID  string
	InboxID    *ids.UUID
}

/*
Connect connects the account picked from a sign-in and subscribes it,
so Meta delivers its messages here.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSocialSignupExpired],
[exceptions.ErrSocialAccountNotFound], [exceptions.ErrChannelAlreadyConnected],
[exceptions.ErrChannelConnectedElsewhere], [exceptions.ErrSocialUnavailable].
*/
func (s *Social) Connect(ctx context.Context, in ConnectSocialInput) (models.ChannelConnection, error) {
	m := in.Membership
	if !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	secret, err := s.recall(ctx, m.Workspace.ID, in.Grant)
	if err != nil {
		return models.ChannelConnection{}, err
	}

	accounts, err := s.shared(ctx, m.Workspace.ID, secret.Token)
	if err != nil {
		return models.ChannelConnection{}, err
	}

	i := slices.IndexFunc(accounts, func(a SocialAccount) bool { return a.ID == in.AccountID })
	if i < 0 {
		return models.ChannelConnection{}, exceptions.ErrSocialAccountNotFound
	}

	account := accounts[i]
	secrets := SocialSecrets{AccessToken: secret.Token, BusinessToken: secret.Token}

	if s.Platform == social.Facebook {
		if secrets.AccessToken, err = s.pageToken(ctx, account, secret.Token); err != nil {
			return models.ChannelConnection{}, err
		}
	}

	conn, err := s.store(ctx, in, social.Facts{
		AccountID: account.ID, Name: account.Name, Username: account.Username, PictureURL: account.PictureURL,
		BusinessID: secret.BusinessID, AppID: s.App.AppID,
	}, secrets)
	if err != nil {
		return conn, err
	}

	return conn, s.subscribe(ctx, account.ID, secrets.AccessToken)
}

/*
pageToken returns the Page's own token: the one the sign-in listed it
with, or the one Meta trades the business's token for.
*/
func (s *Social) pageToken(ctx context.Context, account SocialAccount, business string) (string, error) {
	if account.token != "" {
		return account.token, nil
	}

	var out struct {
		Token string `json:"access_token"`
	}

	if err := s.graph(business).Get(ctx, account.ID, url.Values{"fields": {"access_token"}}, &out); err != nil {
		return "", s.refused(err)
	}

	if out.Token == "" {
		return "", exceptions.ErrSocialAccountNotFound
	}

	return out.Token, nil
}

/*
subscribe asks Meta to deliver the account's events here.
*/
func (s *Social) subscribe(ctx context.Context, account, token string) error {
	fields := facebookFields
	if s.Platform == social.Instagram {
		fields = instagramFields
	}

	path := account + "/subscribed_apps?" + url.Values{"subscribed_fields": {fields}}.Encode()
	if err := s.graph(token).Post(ctx, path, map[string]any{}, nil); err != nil {
		return s.unavailable(err)
	}

	return nil
}

/*
store keeps the connection, live, with its sealed secrets, refusing an
account already connected here or in another workspace.
*/
func (s *Social) store(ctx context.Context, in ConnectSocialInput, facts social.Facts, secrets SocialSecrets) (
	models.ChannelConnection, error,
) {
	config, err := json.Marshal(facts)
	if err != nil {
		return models.ChannelConnection{}, fmt.Errorf("encode %s config: %w", s.Platform, err)
	}

	var conn models.ChannelConnection

	ws := in.Membership.Workspace.ID

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inbox, err := inboxFor(ctx, q, ws, in.InboxID)
		if err != nil {
			return err
		}

		conn, err = s.insert(ctx, q, models.CreateChannelConnectionParams{
			WorkspaceID: ws, InboxID: inbox, Type: string(s.Platform), Status: "active", ExternalID: &facts.AccountID,
			Name: cmp.Or(facts.Name, facts.Username, s.Platform.Name()), Config: config, Now: s.Clock.Now(),
		})
		if err != nil {
			return err
		}

		sealed, err := s.seal(ctx, conn, secrets)
		if err != nil {
			return err
		}

		conn.CredentialsEncrypted = sealed

		return q.SealChannelConnectionCredentials(ctx, models.SealChannelConnectionCredentialsParams{
			CredentialsEncrypted: sealed, Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
	})

	return conn, err
}

/*
insert adds the live connection, refusing an account connected here or
in another workspace already.
*/
func (s *Social) insert(ctx context.Context, q *models.Queries, row models.CreateChannelConnectionParams) (
	models.ChannelConnection, error,
) {
	_, err := q.FindLiveChannelConnectionByExternalID(ctx, models.FindLiveChannelConnectionByExternalIDParams{
		WorkspaceID: row.WorkspaceID, Type: row.Type, ExternalID: row.ExternalID,
	})
	if err == nil {
		return models.ChannelConnection{}, exceptions.ErrChannelAlreadyConnected
	}

	if !errors.Is(err, database.ErrNotFound) {
		return models.ChannelConnection{}, err
	}

	conn, err := q.CreateChannelConnection(ctx, row)
	if database.IsUniqueViolation(err, "") {
		return conn, exceptions.ErrChannelConnectedElsewhere
	}

	if err != nil {
		return conn, fmt.Errorf("create %s connection: %w", s.Platform, err)
	}

	return conn, nil
}

/*
List returns the workspace's connections of the platform.

Errors: [exceptions.ErrForbidden].
*/
func (s *Social) List(ctx context.Context, m authz.Membership) ([]models.ChannelConnection, error) {
	if !m.Can("settings:read") && !m.Can("channels:manage") {
		return nil, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var out []models.ChannelConnection

	err := s.read(ctx, func(q *models.Queries) error {
		var err error
		out, err = q.ListChannelConnectionsOfType(ctx, models.ListChannelConnectionsOfTypeParams{
			WorkspaceID: m.Workspace.ID, Type: string(s.Platform),
		})

		return err
	})

	return out, err
}

/*
Find returns the workspace's connection of the platform.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (s *Social) Find(ctx context.Context, m authz.Membership, id ids.UUID) (models.ChannelConnection, error) {
	if !m.Can("settings:read") && !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	return s.connection(tenancy.With(ctx, m.Workspace.ID.String()), m.Workspace.ID, id)
}

/*
Update moves the connection's new conversations to another inbox.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (s *Social) Update(ctx context.Context, m authz.Membership, id ids.UUID, inbox *ids.UUID) (
	models.ChannelConnection, error,
) {
	if !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	conn, err := s.connection(ctx, m.Workspace.ID, id)
	if err != nil || inbox == nil {
		return conn, err
	}

	err = s.read(ctx, func(q *models.Queries) error {
		target, err := inboxFor(ctx, q, m.Workspace.ID, inbox)
		if err != nil {
			return err
		}

		conn, err = q.MoveChannelConnection(ctx, models.MoveChannelConnectionParams{
			InboxID: target, Now: s.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return err
	})

	return conn, err
}

/*
Disconnect retires the connection: Meta stops delivering its events
here, its tokens go, and its conversations stay.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (s *Social) Disconnect(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("channels:manage") {
		return exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	conn, err := s.connection(ctx, m.Workspace.ID, id)
	if err != nil {
		return err
	}

	if token, err := s.Token(ctx, conn); err == nil {
		_ = s.graph(token).Delete(ctx, SocialFactsOf(conn).AccountID+"/subscribed_apps")
	}

	return s.read(ctx, func(q *models.Queries) error {
		_, err := q.DisconnectChannelConnection(ctx, models.DisconnectChannelConnectionParams{
			Now: s.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return err
	})
}
