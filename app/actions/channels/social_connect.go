/*
|--------------------------------------------------------------------------
| Connecting Pages And Instagram Accounts
|--------------------------------------------------------------------------
|
| A workspace connects through Meta's own sign-in: a popup where an
| admin picks a business and the Pages or Instagram accounts to share,
| which hands back a code. The code becomes the business's token, the
| accounts it shared are listed for the admin to pick from, and the one
| picked is connected: kept with its sealed tokens and subscribed, so
| Meta delivers its messages here. The token waits between the two
| steps sealed in the cache for a few minutes, named by a grant only the
| workspace that signed in can use.
|
| Messenger sends with a Page's own token, which the business's token
| is traded for. Instagram sends with the business's token itself.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
socialGrantTTL is how long a sign-in waits for the admin to pick an
account.
*/
const socialGrantTTL = 15 * time.Minute

/*
The events a connected account's deliveries carry.
*/
const (
	facebookFields = "messages,message_echoes,messaging_postbacks,message_reactions,message_reads," +
		"message_edits,messaging_handovers,standby"
	instagramFields = "messages,message_reactions,messaging_postbacks,messaging_seen"
)

/*
SocialSignupConfig is what Meta's popup needs to open, and whether this
install can open it.
*/
type SocialSignupConfig struct {
	Available    bool
	AppID        string
	ConfigID     string
	GraphVersion string
}

/*
SignupConfig returns what the platform's popup opens with.
*/
func (s *Social) SignupConfig() SocialSignupConfig {
	return SocialSignupConfig{
		Available: s.App.Configured() && s.Login.ConfigID != "", AppID: s.App.AppID, ConfigID: s.Login.ConfigID,
		GraphVersion: s.Config.GraphVersion,
	}
}

/*
SocialAccount is a Page or Instagram account a sign-in shared.
Connected says the workspace already connected it.
*/
type SocialAccount struct {
	ID         string
	Name       string
	Username   string
	PictureURL string
	Connected  bool

	token string
}

/*
SocialGrant is a sign-in waiting for its account to be picked: the
grant that names it and the accounts it shared.
*/
type SocialGrant struct {
	Grant    string
	Accounts []SocialAccount
}

/*
grantSecret is what a grant keeps sealed: the business's token and the
business it is of.
*/
type grantSecret struct {
	Token      string `json:"token"`
	BusinessID string `json:"business_id"`
}

/*
SocialSignupInput is what Meta's popup handed back: the code and the
address it was returned to, which Meta checks the code against.
*/
type SocialSignupInput struct {
	Code        string
	RedirectURI string
}

/*
Signup trades the popup's code for the business's token and returns the
accounts it shared, under a grant to connect one with.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSocialSignupUnavailable],
[exceptions.ErrSocialSignupExpired], [exceptions.ErrSocialUnavailable].
*/
func (s *Social) Signup(ctx context.Context, m authz.Membership, in SocialSignupInput) (SocialGrant, error) {
	if !m.Can("channels:manage") {
		return SocialGrant{}, exceptions.ErrForbidden
	}

	if !s.SignupConfig().Available {
		return SocialGrant{}, exceptions.ErrSocialSignupUnavailable
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	token, err := s.exchange(ctx, in)
	if err != nil {
		return SocialGrant{}, err
	}

	accounts, err := s.shared(ctx, m.Workspace.ID, token)
	if err != nil {
		return SocialGrant{}, err
	}

	grant, err := s.remember(ctx, m.Workspace.ID, grantSecret{Token: token, BusinessID: s.business(ctx, token)})
	if err != nil {
		return SocialGrant{}, err
	}

	s.inlinePictures(ctx, accounts)

	return SocialGrant{Grant: grant, Accounts: accounts}, err
}

/*
exchange swaps the popup's code for the business's token.
*/
func (s *Social) exchange(ctx context.Context, in SocialSignupInput) (string, error) {
	if strings.TrimSpace(in.Code) == "" {
		return "", exceptions.ErrSocialSignupExpired
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}

	query := url.Values{"client_id": {s.App.AppID}, "client_secret": {s.App.AppSecret}, "code": {in.Code}}

	if in.RedirectURI != "" {
		if !strings.HasPrefix(in.RedirectURI, strings.TrimRight(s.AppURL, "/")+"/") {
			return "", exceptions.ErrSocialSignupExpired
		}

		query.Set("redirect_uri", in.RedirectURI)
	}

	err := s.graph("").Get(ctx, "oauth/access_token", query, &out)
	if refused, ok := errors.AsType[*meta.GraphError](err); ok && refused.Status < 500 {
		return "", exceptions.ErrSocialSignupExpired
	}

	if err != nil {
		return "", s.unavailable(err)
	}

	if out.AccessToken == "" {
		return "", exceptions.ErrSocialSignupExpired
	}

	return out.AccessToken, nil
}

/*
business reads which business the token is of, empty when Meta does not
say.
*/
func (s *Social) business(ctx context.Context, token string) string {
	var out struct {
		BusinessID string `json:"client_business_id"`
	}

	_ = s.graph(token).Get(ctx, "me", url.Values{"fields": {"client_business_id"}}, &out)

	return out.BusinessID
}

/*
shared lists the Pages or Instagram accounts the token was given, each
marked when the workspace connected it already.
*/
func (s *Social) shared(ctx context.Context, ws ids.UUID, token string) ([]SocialAccount, error) {
	edge, fields := "me/assigned_pages", "id,name,picture{url},access_token"
	if s.Platform == social.Instagram {
		edge, fields = "me/assigned_instagram_accounts", "id,username,name,profile_picture_url"
	}

	var out struct {
		Data []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Token    string `json:"access_token"`
			Avatar   string `json:"profile_picture_url"`
			Picture  struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			} `json:"picture"`
		} `json:"data"`
	}

	if err := s.graph(token).Get(ctx, edge, url.Values{"fields": {fields}, "limit": {"100"}}, &out); err != nil {
		return nil, s.refused(err)
	}

	connected, err := s.connectedAccounts(ctx, ws)
	if err != nil {
		return nil, err
	}

	accounts := make([]SocialAccount, 0, len(out.Data))
	for _, a := range out.Data {
		accounts = append(accounts, SocialAccount{
			ID: a.ID, Name: firstOf(a.Name, a.Username), Username: a.Username,
			PictureURL: firstOf(a.Avatar, a.Picture.Data.URL), Connected: slices.Contains(connected, a.ID),
			token: a.Token,
		})
	}

	return accounts, nil
}

/*
remember seals the secret in the cache for a few minutes and returns
the grant that opens it again.
*/
func (s *Social) remember(ctx context.Context, ws ids.UUID, secret grantSecret) (string, error) {
	grant := strings.ToLower(str.Random(32))

	sealed, err := encryption.Seal(ctx, s.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: ws.String()}, AAD: []byte(s.grantKey(ws, grant)),
	}, secret)
	if err != nil {
		return "", fmt.Errorf("seal %s grant: %w", s.Platform, err)
	}

	if err := s.Cache.Set(ctx, s.grantKey(ws, grant), sealed, socialGrantTTL); err != nil {
		return "", fmt.Errorf("keep %s grant: %w", s.Platform, err)
	}

	return grant, nil
}

/*
recall opens a grant of the workspace, expired when it is gone.
*/
func (s *Social) recall(ctx context.Context, ws ids.UUID, grant string) (grantSecret, error) {
	sealed, err := s.Cache.Get(ctx, s.grantKey(ws, grant))
	if err != nil || grant == "" {
		return grantSecret{}, exceptions.ErrSocialSignupExpired.OnField("grant")
	}

	secret, err := encryption.Open[grantSecret](ctx, s.Keys, sealed, []byte(s.grantKey(ws, grant)))
	if err != nil {
		return grantSecret{}, exceptions.ErrSocialSignupExpired.OnField("grant")
	}

	return secret, nil
}

func (s *Social) grantKey(ws ids.UUID, grant string) string {
	return "ch:grant:" + string(s.Platform) + ":" + ws.String() + ":" + grant
}

/*
unavailable reports Meta not answering, keeping the cause for the logs.
*/
func (s *Social) unavailable(err error) error {
	return fmt.Errorf("%w: %w", exceptions.ErrSocialUnavailable, err)
}

/*
refused reads a failed call: a sign-in Meta no longer takes when Meta
refused it, Meta not answering otherwise.
*/
func (s *Social) refused(err error) error {
	if refused, ok := errors.AsType[*meta.GraphError](err); ok && refused.Status < 500 {
		return fmt.Errorf("%w: %w", exceptions.ErrSocialSignupExpired, err)
	}

	return s.unavailable(err)
}

/*
connectedAccounts returns the accounts the workspace connected.
*/
func (s *Social) connectedAccounts(ctx context.Context, ws ids.UUID) ([]string, error) {
	var out []string

	err := s.read(ctx, func(q *models.Queries) error {
		rows, err := q.ListChannelConnectionsOfType(ctx, models.ListChannelConnectionsOfTypeParams{
			WorkspaceID: ws, Type: string(s.Platform),
		})
		for _, row := range rows {
			out = append(out, SocialFactsOf(row).AccountID)
		}

		return err
	})

	return out, err
}
