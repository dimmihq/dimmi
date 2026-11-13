/*
|--------------------------------------------------------------------------
| Catalog Reads For Operator
|--------------------------------------------------------------------------
|
| Operator reaches the API's catalog through its tool layer, but only to
| read. A tool here is a read-only operation of the catalog, called the
| same way an agent's tool is: the request the REST API would receive,
| sent to the same handler with the teammate's own credential, so the
| teammate's role and the workspace's rules apply unchanged. Writes, and
| reads of private data that need the person's approval on every call,
| stay out of reach, each listed with the reason.
|
*/

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
ReadTool is one read-only catalog tool: its name, description, input
schema, and the group it belongs to, the first part of its operation.
*/
type ReadTool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Group       string
}

/*
Left is a catalog tool kept out of Operator's reach, and why.
*/
type Left struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

/*
Reasons a catalog tool is left out.
*/
const (
	leftWrites    = "changes data; Operator changes the workspace only through its own changesets"
	leftSensitive = "reads private data or reaches out, which the person must approve on every call"
)

/*
Reads are the catalog's read-only tools and the handler that serves
them.
*/
type Reads struct {
	tools []ReadTool
	left  []Left
	ops   map[string]mcp.Operation
	docs  map[string]Doc
	api   fhttp.Handler
}

/*
NewReads builds the read-only tools from the router's API.
*/
func NewReads(opts Options) (*Reads, error) {
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
	r := &Reads{ops: map[string]mcp.Operation{}, docs: catalog(), api: api}
	for _, op := range ops {
		d, ok := r.docs[op.ID]
		if !ok {
			continue
		}
		def := definition(op, d)
		switch {
		case !def.ReadOnly && op.Method == fhttp.MethodGet:
			r.left = append(r.left, Left{Name: def.Name, Reason: leftSensitive})
			continue
		case !def.ReadOnly:
			r.left = append(r.left, Left{Name: def.Name, Reason: leftWrites})
			continue
		}
		schema, err := json.Marshal(inputSchema(op, d))
		if err != nil {
			return nil, fmt.Errorf("encode schema of %s: %w", op.ID, err)
		}
		r.ops[def.Name] = op
		r.tools = append(r.tools, ReadTool{
			Name: def.Name, Description: def.Description, Schema: schema, Group: groupOf(op.ID),
		})
	}
	slices.SortFunc(r.tools, func(a, b ReadTool) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(r.left, func(a, b Left) int { return strings.Compare(a.Name, b.Name) })
	return r, nil
}

/*
groupOf is an operation's group: the resource it starts with.
*/
func groupOf(operationID string) string {
	head, _, _ := strings.Cut(operationID, ".")
	return "api_" + strings.ReplaceAll(head, "-", "_")
}

/*
Tools are the read-only tools, by name.
*/
func (r *Reads) Tools() []ReadTool {
	return slices.Clone(r.tools)
}

/*
Left are the tools kept out of reach, by name.
*/
func (r *Reads) Left() []Left {
	return slices.Clone(r.left)
}

/*
ReadCall is one read: the tool, its arguments, the caller's credential
as an Authorization header's value, and whether to return the API's
JSON body, for the caller to trim, instead of the text an agent reads.
*/
type ReadCall struct {
	Name          string
	Args          map[string]any
	Authorization string
	Raw           bool
}

/*
Call runs a read-only tool with the caller's credential.
*/
func (r *Reads) Call(ctx context.Context, in ReadCall) (string, error) {
	name, args := in.Name, in.Args
	op, ok := r.ops[name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUndocumented, name)
	}
	header := fhttp.Header{}
	header.Set("Authorization", in.Authorization)
	a, err := op.Call(ctx, mcp.API{Handler: r.api, Prefix: apiPrefix}, args, mcp.Origin{Header: header})
	if err != nil {
		return "", err
	}
	doc := r.docs[op.ID]
	if a.Status >= fhttp.StatusBadRequest {
		return "", refusal(name, doc, op, a)
	}
	if in.Raw {
		return string(a.Body), nil
	}
	return render(doc, name, a).Text(), nil
}
