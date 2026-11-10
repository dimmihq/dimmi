/*
|--------------------------------------------------------------------------
| The MCP Server
|--------------------------------------------------------------------------
|
| dimmi's MCP server is built from its own API: every operation a token
| can call becomes a tool, described from the catalog, checked against
| the same schema, and run through the API handler itself. Operations
| that need a person at a browser are left out, each with its reason.
| The server is built the first time an agent connects, once every route
| is declared, so it always matches the API the binary serves.
|
*/

package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/routing"
)

/*
ErrUndocumented marks an API operation offered as a tool without a
catalog entry.
*/
var ErrUndocumented = errors.New("mcpserver: operation has no catalog entry")

/*
Options are what the server is built from: the router whose API it
offers, the handler that serves that API, the sealer of confirmation
tokens, the clock, the error renderer and the version it reports.
*/
type Options struct {
	Router  *routing.Router
	API     func() (fhttp.Handler, error)
	Sealer  encryption.Encrypter
	Clock   clock.Clock
	Errors  fhttp.Errors
	Version string
}

/*
New builds the server: every callable operation of the API as a tool,
and the guide.
*/
func New(opts Options) (*mcp.Server, error) {
	doc, err := opts.Router.OpenAPI(apiPrefix)
	if err != nil {
		return nil, fmt.Errorf("read api document: %w", err)
	}

	ops, err := mcp.Operations(doc, callable(opts.Router))
	if err != nil {
		return nil, fmt.Errorf("read api operations: %w", err)
	}

	api, err := opts.API()
	if err != nil {
		return nil, fmt.Errorf("build api handler: %w", err)
	}

	docs := catalog()
	titles := map[string]string{}

	for _, op := range ops {
		titles[toolName(op.ID)] = docs[op.ID].Title
	}

	s := mcp.New(mcp.ServerOptions{
		Name: "dimmi", Title: "dimmi", Version: opts.Version, Instructions: instructions, Errors: opts.Errors,
		Confirm: approvals{sealer: opts.Sealer, clock: opts.Clock, titles: titles}.decide,
	})

	if err := addGuide(s); err != nil {
		return nil, err
	}

	for _, op := range ops {
		if err := register(s, api, op, docs); err != nil {
			return nil, err
		}
	}

	return s, nil
}

/*
register adds the tool of one operation.
*/
func register(s *mcp.Server, api fhttp.Handler, op mcp.Operation, docs map[string]Doc) error {
	doc, ok := docs[op.ID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUndocumented, op.ID)
	}

	def := definition(op, doc)

	err := mcp.SchemaTool(s, def, inputSchema(op, doc), func(ctx context.Context, args map[string]any) (any, error) {
		delete(args, confirmArgument)

		a, err := call(ctx, api, op, args)
		if err != nil {
			return nil, err
		}

		if a.Status >= fhttp.StatusBadRequest {
			return nil, refusal(def.Name, doc, op, a)
		}

		return render(doc, def.Name, a), nil
	})
	if err != nil {
		return fmt.Errorf("register %s: %w", op.ID, err)
	}

	return nil
}

/*
callable keeps the operations an agent can complete with a token: those
open to every credential, access tokens included, that need neither a
recently confirmed password, a file upload nor a person at a browser.
An operation kept for the person's own browser or app is left out, so
no agent sees a tool it could never use.
*/
func callable(router *routing.Router) func(mcp.Operation) bool {
	routes := map[string]*routing.Route{}
	for _, route := range router.Routes() {
		routes[route.Name()] = route
	}

	skip := excluded()

	return func(op mcp.Operation) bool {
		route, ok := routes[op.ID]
		if !ok {
			return false
		}

		if _, out := skip[op.ID]; out || op.Upload {
			return false
		}

		names := route.MiddlewareNames()

		return slices.Contains(names, "auth") && !slices.Contains(names, "auth:user") &&
			!slices.Contains(names, "password.confirm")
	}
}

/*
Handler serves the server over HTTP once it is built, building it on the
first request. Each request keeps what every call must repeat of it.
*/
func Handler(build func() (*mcp.Server, error), opts mcp.Options, errs fhttp.Errors) fhttp.Handler {
	var (
		once    sync.Once
		handler fhttp.Handler
		failure error
	)

	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		once.Do(func() {
			s, err := build()
			if err != nil {
				failure = err

				return
			}

			handler = s.HTTPHandler(opts)
		})

		if failure != nil {
			errs.Render(w, r, failure)

			return
		}

		handler.ServeHTTP(w, r.WithContext(withOrigin(r.Context(), r)))
	})
}
