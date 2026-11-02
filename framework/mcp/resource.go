/*
|--------------------------------------------------------------------------
| Resources and Prompts
|--------------------------------------------------------------------------
|
| Resources are documents an agent can read by URI, and prompts are
| message templates a person can pick in their client. Both follow the
| same visibility rules as tools: a caller who may not see one cannot
| list it, and reading it answers as if it did not exist.
|
*/

package mcp

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
ResourceDef describes a resource.

URI is the resource's absolute URI, or a URI template such as
"files://{name}" standing for a family of resources. Name, Title and
Description describe it and MIMEType is the type of its text. Toolset
and Abilities decide who may see it, as for tools.
*/
type ResourceDef struct {
	URI         string
	Name        string
	Title       string
	Description string
	MIMEType    string
	Toolset     string
	Abilities   []string
}

/*
PromptDef describes a prompt.

Name identifies it, Title names it for people, Description says what it
produces and Arguments lists what the person fills in. Toolset and
Abilities decide who may see it, as for tools.
*/
type PromptDef struct {
	Name        string
	Title       string
	Description string
	Arguments   []Argument
	Toolset     string
	Abilities   []string
}

/*
Argument is a value a person fills in for a prompt.
*/
type Argument struct {
	Name        string
	Description string
	Required    bool
}

/*
Resource registers the function as the resource. The function receives
the values a template's variables matched, none for a plain URI, and
returns the resource's text.
*/
func Resource(
	s *Server, def ResourceDef, fn func(ctx context.Context, params map[string]string) (string, error),
) error {
	parsed, err := url.Parse(def.URI)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("%w: resource %q is not an absolute URI", ErrInvalidDefinition, def.URI)
	}

	template, err := uritemplate.New(def.URI)
	if err != nil {
		return fmt.Errorf("%w: resource %q: %w", ErrInvalidDefinition, def.URI, err)
	}

	a := access{toolset: def.Toolset, abilities: def.Abilities}
	s.register(s.resources, def.URI, a)

	handler := func(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		uri := req.Params.URI
		if !allowed(ctx, a) {
			return nil, sdk.ResourceNotFoundError(uri)
		}

		text, err := fn(ctx, matches(template, uri))
		if err != nil {
			return nil, s.protocolError(ctx, err)
		}

		return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{
			{URI: uri, MIMEType: def.MIMEType, Text: text},
		}}, nil
	}

	if len(template.Varnames()) == 0 {
		s.sdk.AddResource(&sdk.Resource{
			URI: def.URI, Name: def.Name, Title: def.Title, Description: def.Description, MIMEType: def.MIMEType,
		}, handler)

		return nil
	}

	s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: def.URI, Name: def.Name, Title: def.Title, Description: def.Description, MIMEType: def.MIMEType,
	}, handler)

	return nil
}

/*
matches returns the values the template's variables take in the URI.
*/
func matches(template *uritemplate.Template, uri string) map[string]string {
	params := map[string]string{}

	for name, value := range template.Match(uri) {
		params[name] = value.String()
	}

	return params
}

/*
Prompt registers the function as the prompt. The function receives the
arguments the person filled in and returns the text of the user message
the prompt produces.
*/
func Prompt(s *Server, def PromptDef, fn func(ctx context.Context, args map[string]string) (string, error)) {
	a := access{toolset: def.Toolset, abilities: def.Abilities}
	s.register(s.prompts, def.Name, a)

	arguments := make([]*sdk.PromptArgument, 0, len(def.Arguments))
	for _, arg := range def.Arguments {
		arguments = append(arguments, &sdk.PromptArgument{
			Name: arg.Name, Description: arg.Description, Required: arg.Required,
		})
	}

	prompt := &sdk.Prompt{Name: def.Name, Title: def.Title, Description: def.Description, Arguments: arguments}

	s.sdk.AddPrompt(prompt, func(ctx context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
		if !allowed(ctx, a) {
			return nil, invalidParams("unknown prompt %q", def.Name)
		}

		text, err := fn(ctx, req.Params.Arguments)
		if err != nil {
			return nil, s.protocolError(ctx, err)
		}

		return &sdk.GetPromptResult{
			Description: def.Description,
			Messages:    []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: text}}},
		}, nil
	})
}

/*
protocolError turns a resource or prompt function's error into a
protocol error: a fault's message as invalid parameters, anything else
reported and hidden behind an internal error.
*/
func (s *Server) protocolError(ctx context.Context, err error) error {
	res := s.errors.Resolve(ctx, err)
	if res.Status >= http.StatusInternalServerError {
		return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: res.Message}
	}

	return invalidParams("%s", res.Message)
}
