/*
|--------------------------------------------------------------------------
| MCP Server
|--------------------------------------------------------------------------
|
| One server holds every tool, resource and prompt the application offers
| agents, and every transport serves that same registry. What a caller
| sees is decided per request: features of a toolset stay hidden until
| the caller enables the toolset, and features needing abilities stay
| hidden from callers whose credential lacks them. Lists are filtered by
| caller, so they are never cached as if every caller saw the same.
|
*/

package mcp

import (
	"context"
	"errors"
	"slices"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
)

/*
AllToolsets is the toolset name that enables every toolset.
*/
const AllToolsets = "all"

/*
ErrUnknownToolset is the fault for a toolset no feature was registered
under. The response lists the valid names in toolsets.
*/
var ErrUnknownToolset = fault.New(fault.Invalid, "unknown_toolset", "The toolset does not exist.")

/*
ErrInvalidDefinition is returned when a tool, resource or prompt cannot
be registered as defined, such as a tool whose input is not an object.
*/
var ErrInvalidDefinition = errors.New("mcp: invalid definition")

/*
ServerOptions configure a server.

Name, Title and Version identify the server to clients, and
Instructions tell agents how to use it. Errors turns failed handlers
into results and is used to render refused HTTP requests. Confirm
decides whether a tool that needs confirmation runs; without it such a
tool always runs.
*/
type ServerOptions struct {
	Name         string
	Title        string
	Version      string
	Instructions string
	Errors       http.Errors
	Confirm      Confirmer
}

/*
Server holds the registered features and serves them over HTTP or
stdio. Features are registered before the server starts serving.
*/
type Server struct {
	sdk       *sdk.Server
	errors    http.Errors
	confirm   Confirmer
	toolsets  []string
	tools     map[string]access
	resources map[string]access
	prompts   map[string]access
}

/*
access is who may see a feature: callers that enabled its toolset, or
any caller when it has none, holding every ability it lists.
*/
type access struct {
	toolset   string
	abilities []string
}

/*
toolsetsKey is the context key under which the enabled toolsets travel.
*/
type toolsetsKey struct{}

/*
New returns a server with no features.
*/
func New(opts ServerOptions) *Server {
	s := &Server{
		errors:    opts.Errors,
		confirm:   opts.Confirm,
		tools:     map[string]access{},
		resources: map[string]access{},
		prompts:   map[string]access{},
	}

	s.sdk = sdk.NewServer(
		&sdk.Implementation{Name: opts.Name, Title: opts.Title, Version: opts.Version},
		&sdk.ServerOptions{
			Instructions: opts.Instructions,
			Capabilities: &sdk.ServerCapabilities{},
			SetCacheable: privateCache,
		},
	)
	s.sdk.AddReceivingMiddleware(s.filterLists)

	return s
}

/*
Toolsets returns the names of the toolsets features were registered
under, sorted.
*/
func (s *Server) Toolsets() []string {
	return slices.Clone(s.toolsets)
}

/*
WithToolsets returns a copy of the context in which the named toolsets
are enabled for the caller. AllToolsets enables every one. A name no
feature was registered under returns ErrUnknownToolset.
*/
func (s *Server) WithToolsets(ctx context.Context, names ...string) (context.Context, error) {
	enabled := map[string]bool{}

	for _, name := range names {
		if name == AllToolsets {
			for _, toolset := range s.toolsets {
				enabled[toolset] = true
			}

			continue
		}

		if !slices.Contains(s.toolsets, name) {
			return ctx, ErrUnknownToolset.With("toolset", name).With("toolsets", s.Toolsets())
		}

		enabled[name] = true
	}

	return context.WithValue(ctx, toolsetsKey{}, enabled), nil
}

/*
register records who may see a feature and makes its toolset known.
*/
func (s *Server) register(features map[string]access, key string, a access) {
	features[key] = a

	if a.toolset != "" && !slices.Contains(s.toolsets, a.toolset) {
		s.toolsets = append(s.toolsets, a.toolset)
		slices.Sort(s.toolsets)
	}
}

/*
enabled reports whether the caller in the context enabled the feature's
toolset. A feature outside any toolset is always enabled.
*/
func enabled(ctx context.Context, a access) bool {
	if a.toolset == "" {
		return true
	}

	toolsets, _ := ctx.Value(toolsetsKey{}).(map[string]bool)

	return toolsets[a.toolset]
}

/*
missingAbility returns the first ability the feature needs that the
caller in the context lacks, or "" when it lacks none. A context without
a caller holds no ability.
*/
func missingAbility(ctx context.Context, a access) string {
	p, ok := auth.From(ctx)

	for _, ability := range a.abilities {
		if !ok || !auth.HasAbility(p, ability) {
			return ability
		}
	}

	return ""
}

/*
allowed reports whether the caller in the context may see the feature.
*/
func allowed(ctx context.Context, a access) bool {
	return enabled(ctx, a) && missingAbility(ctx, a) == ""
}

/*
hidden reports whether the caller in the context may not see the
feature registered under the key. Unregistered features are hidden.
*/
func hidden(ctx context.Context, features map[string]access, key string) bool {
	a, ok := features[key]

	return !ok || !allowed(ctx, a)
}

/*
filterLists removes from every list result the features the caller may
not see.
*/
func (s *Server) filterLists(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		res, err := next(ctx, method, req)
		if err != nil {
			return res, err
		}

		switch list := res.(type) {
		case *sdk.ListToolsResult:
			list.Tools = slices.DeleteFunc(list.Tools, func(t *sdk.Tool) bool {
				return hidden(ctx, s.tools, t.Name)
			})
		case *sdk.ListResourcesResult:
			list.Resources = slices.DeleteFunc(list.Resources, func(r *sdk.Resource) bool {
				return hidden(ctx, s.resources, r.URI)
			})
		case *sdk.ListResourceTemplatesResult:
			list.ResourceTemplates = slices.DeleteFunc(list.ResourceTemplates, func(r *sdk.ResourceTemplate) bool {
				return hidden(ctx, s.resources, r.URITemplate)
			})
		case *sdk.ListPromptsResult:
			list.Prompts = slices.DeleteFunc(list.Prompts, func(p *sdk.Prompt) bool {
				return hidden(ctx, s.prompts, p.Name)
			})
		}

		return res, nil
	}
}

/*
privateCache marks cacheable results as private to the caller, because
what a caller sees depends on who they are.
*/
func privateCache(_ context.Context, _ sdk.Request, c *sdk.Cacheable) {
	if c.CacheScope == "" {
		c.CacheScope = "private"
	}
}
