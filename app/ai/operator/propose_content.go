/*
|--------------------------------------------------------------------------
| Proposals Beyond Conversations
|--------------------------------------------------------------------------
|
| Changes to customers, help articles and procedures. A customer is
| named by id or found through the conversation; an article draft goes
| to the help center named, else the default; a procedure edit replaces
| the whole working copy, which the editor shows before anyone
| publishes it.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxArticleBody is the longest article body Operator writes, in bytes.
*/
const maxArticleBody = 200 << 10

/*
contactOp checks a change to a customer.
*/
func (r resolver) contactOp(ctx context.Context, in opInput) (Operation, error) {
	op := Operation{Kind: in.Kind}
	var contact ids.UUID
	if in.Contact != "" {
		id, err := r.e.publicID(in.Contact, models.PrefixContact)
		if err != nil {
			return op, err
		}
		contact = id
	} else {
		c, err := r.conversation(ctx, in.Conversation)
		if err != nil {
			return op, err
		}
		contact = c.ContactID
	}
	found, err := r.q.FindContact(ctx, models.FindContactParams{WorkspaceID: r.s.m.Workspace.ID, ID: contact})
	if errors.Is(err, database.ErrNotFound) || (err == nil && found.DeletedAt != nil) {
		return op, exceptions.ErrNotFound
	}
	if err != nil {
		return op, fmt.Errorf("find contact: %w", err)
	}
	op.Contact = &contact
	return textOp(op, in.Name, 200, "Rename the customer to “%s”")
}

/*
articleOp checks a new article draft or an edit of one.
*/
func (r resolver) articleOp(ctx context.Context, in opInput) (Operation, error) {
	op := Operation{Kind: in.Kind}
	title, body := strings.TrimSpace(in.Title), strings.TrimSpace(in.Body)
	if len([]rune(title)) > 150 || len(body) > maxArticleBody {
		return op, fmt.Errorf("%w: titles are at most 150 characters and bodies at most 200 KB", errBadArguments)
	}
	if title != "" {
		op.Title = &title
	}
	if body != "" {
		op.Body = &body
	}
	if in.Kind == KindArticleNew {
		return r.newArticleOp(ctx, op, in)
	}
	id, err := r.e.publicID(in.Article, models.PrefixArticle)
	if err != nil {
		return op, err
	}
	a, err := r.e.Services.Articles.Show(ctx, r.s.m, id)
	if err != nil {
		return op, err
	}
	if op.Title == nil && op.Body == nil {
		return op, fmt.Errorf("%w: give a new title or body", errBadArguments)
	}
	op.Article, op.Label = &id, fmt.Sprintf("Edit the draft of “%s”", a.Title)
	return op, nil
}

/*
newArticleOp checks a new article draft and its help center.
*/
func (r resolver) newArticleOp(ctx context.Context, op Operation, in opInput) (Operation, error) {
	if op.Title == nil || op.Body == nil {
		return op, fmt.Errorf("%w: a new article needs a title and a body", errBadArguments)
	}
	centers, err := r.e.helpCenters(ctx, r.s)
	if err != nil {
		return op, err
	}
	if len(centers) == 0 {
		return op, fmt.Errorf("%w: this workspace has no help center yet", errBadArguments)
	}
	center := centers[0].ID
	if in.HelpCenter != "" {
		if center, err = r.e.publicID(in.HelpCenter, models.PrefixHelpCenter); err != nil {
			return op, err
		}
	}
	op.HelpCenter, op.Label = &center, fmt.Sprintf("Draft a new article “%s”", *op.Title)
	return op, nil
}

/*
procedureOp checks an edit of a procedure's working copy.
*/
func (r resolver) procedureOp(ctx context.Context, in opInput) (Operation, error) {
	op := Operation{Kind: in.Kind}
	id, err := r.e.publicID(in.Procedure, models.PrefixProcedure)
	if err != nil {
		return op, err
	}
	p, err := r.e.Services.Procedures.Show(ctx, r.s.m, id)
	if err != nil {
		return op, err
	}
	var shape map[string]any
	if len(in.Definition) == 0 || json.Unmarshal(in.Definition, &shape) != nil {
		return op, fmt.Errorf("%w: give the whole working copy as a JSON object", errBadArguments)
	}
	op.Procedure, op.Definition = &id, in.Definition
	op.Label = fmt.Sprintf("Edit the working copy of the procedure “%s”", p.Name)
	return op, nil
}
