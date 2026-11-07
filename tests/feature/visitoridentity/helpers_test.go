package visitoridentity_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with its owner's client and its messenger.
*/
type desk struct {
	t     *testing.T
	tc    *tests.TestCase
	owner *testkit.Client
	app   string
}

/*
newDesk starts the application on a fresh workspace.
*/
func newDesk(t *testing.T) *desk {
	t.Helper()
	tc := tests.New(t)
	d := &desk{t: t, tc: tc, owner: tc.In(tc.Owner)}
	d.inWorkspace(func(ctx context.Context, q *models.Queries) error {
		rows, err := q.ListWidgetSettings(ctx, tc.Workspace.Workspace.ID)
		if err == nil {
			require.Len(t, rows, 1)
			d.app = models.PublicID(models.PrefixWidget, rows[0].ID)
		}
		return err
	})
	return d
}

/*
secret is a shared secret the owner created, with its key id.
*/
type secret struct {
	id    string
	value string
}

/*
newSecret creates a shared secret through the API.
*/
func (d *desk) newSecret() secret {
	d.t.Helper()
	res := d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"name": "Production", "algorithm": "HS256"}).
		AssertCreated()
	return secret{id: res.String("data.id"), value: res.String("data.secret")}
}

/*
sign mints a token the way a host app's server would: HS256 with the
secret as typed, the kid header, and an expiry ten minutes out.
*/
func (s secret) sign(t *testing.T, now time.Time, claims map[string]any) string {
	t.Helper()
	return s.signAs(t, s.id, now, claims)
}

/*
signAs mints a token with the kid given, empty for none.
*/
func (s secret) signAs(t *testing.T, kid string, now time.Time, claims map[string]any) string {
	t.Helper()
	key, err := jwt.NewKey(kid, jwt.HS256, []byte(s.value))
	require.NoError(t, err)
	c := jwt.Claims{IssuedAt: now, Expiry: now.Add(10 * time.Minute), Extra: map[string]any{}}
	for k, v := range claims {
		switch k {
		case "sub":
			c.Subject, _ = v.(string)
		case "exp":
			c.Expiry, _ = v.(time.Time)
		default:
			c.Extra[k] = v
		}
	}
	token, err := jwt.Sign(c, key)
	require.NoError(t, err)
	return token
}

/*
visitor is a browser on the workspace's messenger with its session
token.
*/
type visitor struct {
	d       *desk
	token   string
	id      string
	contact string
}

/*
client is the visitor's browser, sending their bearer token.
*/
func (v *visitor) client() *testkit.Client {
	return v.d.tc.Client().WithHeader("Authorization", "Bearer "+v.token)
}

/*
anonymous starts an anonymous session.
*/
func (d *desk) anonymous() *visitor {
	d.t.Helper()
	res := d.tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": d.app}).AssertCreated()
	return &visitor{
		d: d, token: res.String("data.token"), id: res.String("data.id"),
		contact: res.String("data.contact.id"),
	}
}

/*
signIn starts a signed-in session with the token, handing over the
previous session when one is given.
*/
func (d *desk) signIn(token string, previous *visitor) *testkit.Response {
	d.t.Helper()
	body := testkit.JSON{"app_id": d.app, "user_jwt": token}
	if previous != nil {
		body["previous_token"] = previous.token
	}
	return d.tc.Client().PostJSON("/widget/v1/sessions", body)
}

/*
signedIn starts a signed-in session and returns the visitor.
*/
func (d *desk) signedIn(token string, previous *visitor) *visitor {
	d.t.Helper()
	res := d.signIn(token, previous).AssertCreated()
	return &visitor{
		d: d, token: res.String("data.token"), id: res.String("data.id"),
		contact: res.String("data.contact.id"),
	}
}

/*
inWorkspace runs queries inside the workspace.
*/
func (d *desk) inWorkspace(fn func(ctx context.Context, q *models.Queries) error) {
	d.t.Helper()
	ws := d.tc.Workspace.Workspace.ID
	err := d.tc.App.DB.Transaction(tenancy.With(d.t.Context(), ws.String()),
		func(ctx context.Context, tx *database.Tx) error { return fn(ctx, models.New(tx)) })
	require.NoError(d.t, err)
}

/*
contact reads a contact by its public id.
*/
func (d *desk) contact(public string) models.Contact {
	d.t.Helper()
	id := d.decode(models.PrefixContact, public)
	var out models.Contact
	d.inWorkspace(func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.FindContact(ctx, models.FindContactParams{WorkspaceID: d.tc.Workspace.Workspace.ID, ID: id})
		return err
	})
	return out
}

/*
session reads a widget session by its public id.
*/
func (d *desk) session(public string) models.WidgetSession {
	d.t.Helper()
	id := d.decode(models.PrefixWidgetSession, public)
	var out models.WidgetSession
	d.inWorkspace(func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.FindWidgetSession(ctx, models.FindWidgetSessionParams{
			WorkspaceID: d.tc.Workspace.Workspace.ID, ID: id,
		})
		return err
	})
	return out
}

func (d *desk) decode(prefix models.Prefix, public string) ids.UUID {
	d.t.Helper()
	id, err := d.tc.App.IDs.Decode(string(prefix), public)
	require.NoError(d.t, err)
	return id
}
