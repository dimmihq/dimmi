/*
|--------------------------------------------------------------------------
| Widget Realtime Tokens
|--------------------------------------------------------------------------
|
| A visitor's stream opens with a token in its URL, because a browser's
| event source cannot send headers. So that token is good for one thing
| only, opening a stream, and only for a minute: it is no bearer token for
| the API, and a copy left in a proxy log is useless before anyone reads
| it. The stream it opens receives one channel, the visitor's own contact,
| chosen by the server; nothing the browser asks for changes that, and the
| session is checked again whenever the stream opens or resumes.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
realtimeAudience is the one use a realtime token is good for.
*/
const realtimeAudience = "widget.realtime"

/*
realtimeLeeway is the clock skew a realtime token tolerates.
*/
const realtimeLeeway = 5 * time.Second

/*
RealtimeToken is a stream's connect token, when it stops working, and the
channel the stream receives.
*/
type RealtimeToken struct {
	Token     string
	ExpiresAt time.Time
	Channel   string
}

/*
Realtime issues visitors' connect tokens and opens their streams. Key
signs the tokens, IDs reads the session they name and TTL is how long one works.
*/
type Realtime struct {
	DB    *database.DB
	Clock clock.Clock
	IDs   *ids.Registry
	Key   jwt.Key
	TTL   time.Duration
}

/*
NewRealtimeKey returns the key realtime tokens are signed with, from secret
bytes derived for that use alone.
*/
func NewRealtimeKey(secret []byte) (jwt.Key, error) {
	return jwt.NewKey("widget-realtime", jwt.HS256, secret)
}

/*
Issue returns a connect token for the session's stream.
*/
func (a *Realtime) Issue(s models.WidgetSession) (RealtimeToken, error) {
	now := a.Clock.Now()
	out := RealtimeToken{ExpiresAt: now.Add(a.TTL), Channel: appbroadcasting.WidgetContactChannel(s.ContactID)}

	token, err := jwt.Sign(jwt.Claims{
		Subject: models.PublicID(models.PrefixWidgetSession, s.ID), Audience: []string{realtimeAudience},
		IssuedAt: now, Expiry: out.ExpiresAt,
		Extra: map[string]any{"wsp": s.WorkspaceID.String()},
	}, a.Key)
	if err != nil {
		return out, fmt.Errorf("sign realtime token: %w", err)
	}

	out.Token = token

	return out, nil
}

/*
Resolve opens a visitor's stream from its connect token, and reports false
for a token that is no realtime token, which other resolvers then try.
*/
func (a *Realtime) Resolve(ctx context.Context, token string) (fbroadcasting.Principal, bool, error) {
	if !strings.HasPrefix(token, "eyJ") {
		return fbroadcasting.Principal{}, false, nil
	}

	claims, err := jwt.Verify(token, []jwt.Key{a.Key}, jwt.Options{
		Algorithms: []jwt.Algorithm{jwt.HS256}, Audience: realtimeAudience, Leeway: realtimeLeeway,
		MaxLifetime: a.TTL + realtimeLeeway, Clock: a.Clock,
	})
	if err != nil {
		return fbroadcasting.Principal{}, true, fbroadcasting.ErrUnauthenticated
	}

	session, err := a.session(ctx, claims)
	if err != nil {
		return fbroadcasting.Principal{}, true, err
	}

	if !a.live(session) {
		return fbroadcasting.Principal{}, true, fbroadcasting.ErrUnauthenticated
	}

	userID := models.PublicID(models.PrefixWidgetSession, session.ID)

	return fbroadcasting.Principal{
		UserID: userID, Info: map[string]any{"session_id": userID, "connected_at": a.Clock.Now().UTC()},
		Channels:   []string{appbroadcasting.WidgetContactChannel(session.ContactID)},
		Subject:    &appbroadcasting.WidgetSession{WorkspaceID: session.WorkspaceID, ContactID: session.ContactID},
		Credential: "widget_session:" + session.ID.String(),
		Alive: func(ctx context.Context) (bool, error) {
			current, err := a.find(ctx, session.WorkspaceID, session.ID)
			if errors.Is(err, database.ErrNotFound) {
				return false, nil
			}

			return err == nil && a.live(current), err
		},
	}, true, nil
}

/*
session reads the session a verified token names.
*/
func (a *Realtime) session(ctx context.Context, claims jwt.Claims) (models.WidgetSession, error) {
	workspace, _ := claims.Extra["wsp"].(string)

	ws, err := ids.Parse(workspace)
	if err != nil {
		return models.WidgetSession{}, fbroadcasting.ErrUnauthenticated
	}

	id, err := a.IDs.Decode(string(models.PrefixWidgetSession), claims.Subject)
	if err != nil {
		return models.WidgetSession{}, fbroadcasting.ErrUnauthenticated
	}

	session, err := a.find(ctx, ws, id)
	if errors.Is(err, database.ErrNotFound) {
		return session, fbroadcasting.ErrUnauthenticated
	}

	return session, err
}

/*
find reads one session inside its workspace.
*/
func (a *Realtime) find(ctx context.Context, ws, id ids.UUID) (models.WidgetSession, error) {
	var out models.WidgetSession

	err := a.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).FindWidgetSession(ctx, models.FindWidgetSessionParams{WorkspaceID: ws, ID: id})

		return err
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return out, fmt.Errorf("find widget session: %w", err)
	}

	return out, err
}

/*
live reports whether the session still works: not ended, not expired.
*/
func (a *Realtime) live(s models.WidgetSession) bool {
	return s.RevokedAt == nil && a.Clock.Now().Before(s.ExpiresAt)
}
