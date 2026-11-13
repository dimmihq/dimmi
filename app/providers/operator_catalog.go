/*
|--------------------------------------------------------------------------
| Operator's Catalog Reads
|--------------------------------------------------------------------------
|
| Operator reads the API catalog as the teammate who asked: each call
| carries a short-lived token of the teammate's own, limited to the read
| abilities, so the API's permission checks, the teammate's role and the
| workspace's rules all apply as they would to the teammate's own token.
| The token lives only in memory and expires within the quarter hour.
| The catalog is built on first use, once every route is declared.
|
*/

package providers

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/ai/operator"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/mcpserver"
)

/*
catalogTokenLife is how long a catalog token lives; one is reused until
a minute before it ends.
*/
const catalogTokenLife = 15 * time.Minute

/*
operatorCatalog serves Operator's catalog reads.
*/
type operatorCatalog struct {
	a *app.App

	once  sync.Once
	reads *mcpserver.Reads
	tools []operator.CatalogTool

	mu     sync.Mutex
	tokens map[ids.UUID]minted
}

/*
minted is a member's catalog token and when it ends.
*/
type minted struct {
	plain   string
	expires time.Time
}

/*
newOperatorCatalog returns the catalog over the app's API.
*/
func newOperatorCatalog(a *app.App) *operatorCatalog {
	return &operatorCatalog{a: a, tokens: map[ids.UUID]minted{}}
}

/*
build reads the catalog the first time it is needed; a catalog that
cannot be built offers no tools.
*/
func (c *operatorCatalog) build() {
	c.once.Do(func() {
		reads, err := mcpserver.NewReads(mcpserver.Options{
			Router: c.a.Router, Clock: c.a.Clock, Errors: c.a.Errors, Version: c.a.Version,
			API: func() (http.Handler, error) {
				routes, err := c.a.Router.Handler()
				if err != nil {
					return nil, err
				}
				return c.a.Kernel.Handler(routes), nil
			},
		})
		if err != nil {
			return
		}
		c.reads = reads
		for _, t := range reads.Tools() {
			resource := strings.ReplaceAll(strings.TrimPrefix(t.Group, "api_"), "_", " ")
			c.tools = append(c.tools, operator.CatalogTool{
				Spec:  contracts.ToolSpec{Name: t.Name, Description: t.Description, Schema: t.Schema},
				Group: t.Group, Line: "read-only API: " + resource,
			})
		}
	})
}

/*
Tools are the catalog's read-only tools.
*/
func (c *operatorCatalog) Tools() []operator.CatalogTool {
	c.build()
	return c.tools
}

/*
Left are the catalog tools out of Operator's reach, with the reason.
*/
func (c *operatorCatalog) Left() []mcpserver.Left {
	c.build()
	if c.reads == nil {
		return nil
	}
	return c.reads.Left()
}

/*
Call runs a read-only tool as the teammate.
*/
func (c *operatorCatalog) Call(ctx context.Context, m authz.Membership, call operator.CatalogCall) (string, error) {
	c.build()
	if c.reads == nil {
		return "", mcpserver.ErrUndocumented
	}
	plain, err := c.token(ctx, m)
	if err != nil {
		return "", err
	}
	decoded := map[string]any{}
	if len(call.Args) > 0 {
		if err := json.Unmarshal(call.Args, &decoded); err != nil {
			return "", err
		}
	}
	return c.reads.Call(ctx, mcpserver.ReadCall{
		Name: call.Name, Args: decoded, Authorization: "Bearer " + plain, Raw: call.Raw,
	})
}

/*
token returns the member's live catalog token, minting one when there
is none or it is about to end.
*/
func (c *operatorCatalog) token(ctx context.Context, m authz.Membership) (string, error) {
	now := c.a.Clock.Now()
	c.mu.Lock()
	t, ok := c.tokens[m.Member.ID]
	c.mu.Unlock()
	if ok && now.Add(time.Minute).Before(t.expires) {
		return t.plain, nil
	}
	expires := now.Add(catalogTokenLife)
	var plain string
	scoped := tenancy.With(ctx, m.Workspace.ID.String())
	err := c.a.DB.Transaction(scoped, func(ctx context.Context, _ *database.Tx) error {
		var err error
		_, plain, err = c.a.Tokens.Live.Create(ctx, token.Token{
			OwnerID: m.Member.ID, TenantID: m.Workspace.ID.String(), Name: "Operator catalog reads",
			Abilities: readAbilities(), ExpiresAt: expires,
		})
		return err
	})
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.tokens[m.Member.ID] = minted{plain: plain, expires: expires}
	c.mu.Unlock()
	return plain, nil
}

/*
readAbilities are the ability groups that only read.
*/
func readAbilities() []string {
	var out []string
	for _, a := range appauth.AbilityGroups() {
		if strings.HasSuffix(a, ":read") {
			out = append(out, a)
		}
	}
	return out
}
