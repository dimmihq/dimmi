/*
|--------------------------------------------------------------------------
| Custom View Lists
|--------------------------------------------------------------------------
|
| A saved view lists conversations live: its conditions are compiled into
| the list's query each time it opens, beside whatever filters, search
| and cursor the request adds, so it never holds a stale copy. The view's
| own status and sort apply unless the request chose others. A cursor
| remembers which version of the view it was made for, so editing the
| view sends screens back to the first page instead of skipping rows.
| Conditions not saved yet narrow the list the same way, on top of the
| view it shows, so a teammate can try them before saving.
|
*/

package conversations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
	"github.com/dimmihq/dimmi/app/support/searchquery"
)

/*
ViewCustom is the view of a saved custom view, named by Filters.ViewID.
*/
const ViewCustom = "custom"

/*
FindView returns a live view the member may see, refusing another
member's personal view as not found.
*/
func FindView(
	ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID,
) (models.ConversationView, error) {
	v, err := q.FindConversationView(ctx, models.FindConversationViewParams{
		WorkspaceID: m.Workspace.ID, ID: id, MemberID: m.Member.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return v, exceptions.ErrNotFound
	}

	if err != nil {
		return v, fmt.Errorf("find conversation view: %w", err)
	}

	return v, nil
}

/*
ReadDefinition decodes a stored or sent definition.
*/
func ReadDefinition(raw json.RawMessage) (querybuilder.Definition, error) {
	var d querybuilder.Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, exceptions.ErrInvalidDefinition.With("path", "definition")
	}

	return d, nil
}

/*
NewCompiler returns the compiler of definitions for the member at the
moment.
*/
func NewCompiler(m authz.Membership, registry *ids.Registry, now time.Time) querybuilder.Compiler {
	return querybuilder.Compiler{MemberID: m.Member.ID, Now: now, Decode: registry, Search: searchText}
}

/*
searchText turns the text of a definition's search into a tsquery.
*/
func searchText(text string) (string, error) {
	if len(text) > maxSearchLength {
		return "", errors.New("search for at most 256 characters")
	}

	sq := searchquery.Parse(text)
	if sq.TSQuery == "" {
		return "", errors.New("search words are required")
	}

	return sq.TSQuery, nil
}

/*
maxSearchLength is the longest search a definition holds.
*/
const maxSearchLength = 256

/*
CompileError turns a compiler's refusal into the API's error, naming
where the definition is wrong.
*/
func CompileError(err error) error {
	if def, ok := errors.AsType[*querybuilder.DefinitionError](err); ok {
		return exceptions.ErrInvalidDefinition.With("path", def.Path).With("reason", def.Reason)
	}

	return err
}

/*
customView is what narrows a list beyond its filters: the predicates
and searches of a saved view and of conditions not saved yet, and the
hash of the saved version applied.
*/
type customView struct {
	predicates []querybuilder.Fragment
	searches   []string
	hash       string
}

/*
applyCustomView loads and compiles the view a list names, filling in
its status and sort where the request chose none, then adds the
unsaved conditions the request carries.
*/
func (a *ListConversations) applyCustomView(
	ctx context.Context, q *models.Queries, in *ListConversationsInput,
) (*customView, error) {
	view, err := a.savedView(ctx, q, in)
	if err != nil || !unsaved(in.Filters.Definition) {
		return view, err
	}

	if view == nil {
		view = &customView{}
	}

	return view, a.narrow(view, in.Membership, in.Filters.Definition)
}

/*
unsaved reports whether the list carries conditions not saved yet. A
list kept as JSON, as an export keeps it, holds null when it had none.
*/
func unsaved(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

/*
savedView compiles the saved view a list names, if it names one.
*/
func (a *ListConversations) savedView(
	ctx context.Context, q *models.Queries, in *ListConversationsInput,
) (*customView, error) {
	if in.Filters.View != ViewCustom {
		return nil, nil
	}

	v, err := FindView(ctx, q, in.Membership, in.Filters.ViewID)
	if err != nil {
		return nil, err
	}

	view := &customView{
		hash: hashing.SHA256Hex(append(append([]byte{}, v.Definition...), []byte(v.UpdatedAt.String())...))[:16],
	}
	if err := a.narrow(view, in.Membership, v.Definition); err != nil {
		return nil, err
	}

	viewDefaults(in, v)

	return view, nil
}

/*
narrow compiles a definition and adds what it narrows by to the view.
*/
func (a *ListConversations) narrow(view *customView, m authz.Membership, raw json.RawMessage) error {
	d, err := ReadDefinition(raw)
	if err != nil {
		return err
	}

	compiled, err := NewCompiler(m, a.IDs, a.Clock.Now()).Compile(d)
	if err != nil {
		return CompileError(err)
	}

	view.predicates = append(view.predicates, compiled.Predicate)
	if compiled.Search != nil {
		view.searches = append(view.searches, strings.TrimSpace(*compiled.Search))
	}

	return nil
}

/*
viewDefaults gives the list the view's status and sort when the request
left them out.
*/
func viewDefaults(in *ListConversationsInput, v models.ConversationView) {
	f := &in.Filters

	if v.StatusFilter != nil && len(f.Statuses) == 0 && !f.AnyStatus {
		if *v.StatusFilter == "any" {
			f.AnyStatus = true
		} else {
			f.Statuses = []string{*v.StatusFilter}
		}
	}

	if v.Sort != nil && in.DefaultSort {
		in.Sort = querybuilder.Sort(*v.Sort)
	}
}

/*
apply narrows the list by the view, every search joined to the one
typed.
*/
func (c *customView) apply(l *querybuilder.Conversations) {
	if c == nil {
		return
	}

	l.Extra = append(l.Extra, c.predicates...)

	for _, search := range c.searches {
		if l.TSQuery == nil {
			l.TSQuery = new(search)

			continue
		}

		l.TSQuery = new("(" + *l.TSQuery + ") & (" + search + ")")
	}
}
