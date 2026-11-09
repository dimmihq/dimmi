/*
|--------------------------------------------------------------------------
| Applying Content Changes
|--------------------------------------------------------------------------
|
| Customers, articles and procedures change through the actions their
| editors use. Article and procedure changes only ever touch drafts and
| working copies; publishing stays a teammate's step in the editor. A new
| article Operator drafted is deleted on revert only while it was never
| published; an edit is put back only while the draft still holds what
| Operator wrote.
|
*/

package operator

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
draftCopy is an article's title and body, as kept before and after an
edit.
*/
type draftCopy struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

/*
applyContent applies an operation on a customer, article or procedure.
*/
func (e *Engine) applyContent(ctx context.Context, m authz.Membership, op *Operation) error {
	var err error
	switch op.Kind {
	case KindContact:
		err = e.renameContact(ctx, m, op)
	case KindArticleNew:
		var a models.Article
		a, err = e.Services.Articles.Create(ctx, helpcenters.ArticleInput{
			Membership: m, HelpCenterID: *op.HelpCenter, CollectionID: op.Collection, Title: op.Title, Body: op.Body,
		})
		op.Created = &a.ID
	case KindArticleEdit:
		err = e.editArticle(ctx, m, op)
	case KindAction:
		err = e.runAction(ctx, m, op)
	default:
		err = e.editProcedure(ctx, m, op)
	}
	if err == nil {
		op.Outcome = OutcomeApplied
	}
	return err
}

/*
renameContact renames a customer, keeping the name it replaced.
*/
func (e *Engine) renameContact(ctx context.Context, m authz.Membership, op *Operation) error {
	before, err := e.contact(ctx, m, *op.Contact)
	if err != nil {
		return err
	}
	_, err = e.Services.Contacts.Handle(ctx, contacts.UpdateContactInput{
		Membership: m, ContactID: *op.Contact, Fields: contacts.Fields{Name: op.Value},
	})
	op.Before, op.After = rawJSON(deref(before.Name)), rawJSON(deref(op.Value))
	return err
}

/*
contact reads a customer in the teammate's workspace.
*/
func (e *Engine) contact(ctx context.Context, m authz.Membership, id ids.UUID) (models.Contact, error) {
	var out models.Contact
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).FindContact(ctx, models.FindContactParams{WorkspaceID: m.Workspace.ID, ID: id})
		return err
	})
	return out, err
}

/*
editArticle writes an article's working copy, keeping what it replaced.
*/
func (e *Engine) editArticle(ctx context.Context, m authz.Membership, op *Operation) error {
	a, err := e.Services.Articles.Show(ctx, m, *op.Article)
	if err != nil {
		return err
	}
	after, err := e.Services.Articles.Update(ctx, helpcenters.ArticleInput{
		Membership: m, ID: a.ID, Title: op.Title, Body: op.Body,
	})
	op.Before = rawJSON(draftCopy{Title: a.Title, Body: a.BodyMarkdown})
	op.After = rawJSON(draftCopy{Title: after.Title, Body: after.BodyMarkdown})
	return err
}

/*
editProcedure writes a procedure's working copy, keeping what it
replaced.
*/
func (e *Engine) editProcedure(ctx context.Context, m authz.Membership, op *Operation) error {
	p, err := e.Services.Procedures.Show(ctx, m, *op.Procedure)
	if err != nil {
		return err
	}
	after, err := e.Services.Procedures.Save(ctx, procedures.SaveInput{
		Membership: m, ID: p.ID, LockVersion: p.LockVersion, Definition: op.Definition,
	})
	op.Before, op.After = p.Draft, after.Draft
	return err
}

/*
revertContent undoes an operation on a customer, article or procedure.
*/
func (e *Engine) revertContent(ctx context.Context, m authz.Membership, op *Operation) error {
	reverted, err := e.revertContentOp(ctx, m, *op)
	if err != nil {
		return err
	}
	op.Outcome = OutcomeSkipped
	if reverted {
		op.Outcome = OutcomeReverted
	}
	return nil
}

/*
revertContentOp undoes one content operation, reporting false when the
content moved on since.
*/
func (e *Engine) revertContentOp(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	switch op.Kind {
	case KindContact:
		current, err := e.contact(ctx, m, *op.Contact)
		if err != nil || deref(current.Name) != text(op.After) {
			return false, err
		}
		was := text(op.Before)
		_, err = e.Services.Contacts.Handle(ctx, contacts.UpdateContactInput{
			Membership: m, ContactID: *op.Contact, Fields: contacts.Fields{Name: &was}, ClearName: was == "",
		})
		return err == nil, err
	case KindArticleNew:
		return e.dropDraft(ctx, m, op)
	case KindArticleEdit:
		return e.restoreArticle(ctx, m, op)
	case KindAction:
		return false, nil
	default:
		return e.restoreProcedure(ctx, m, op)
	}
}

/*
dropDraft deletes an article Operator drafted, while it was never
published.
*/
func (e *Engine) dropDraft(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	a, err := e.Services.Articles.Show(ctx, m, *op.Created)
	if err != nil || a.FirstPublishedAt != nil {
		return false, err
	}
	return true, e.Services.Articles.Delete(ctx, m, a.ID)
}

/*
restoreArticle puts an article's working copy back while it still holds
what Operator wrote.
*/
func (e *Engine) restoreArticle(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	a, err := e.Services.Articles.Show(ctx, m, *op.Article)
	if err != nil {
		return false, err
	}
	var was, wrote draftCopy
	_ = json.Unmarshal(op.Before, &was)
	_ = json.Unmarshal(op.After, &wrote)
	if a.Title != wrote.Title || a.BodyMarkdown != wrote.Body {
		return false, nil
	}
	_, err = e.Services.Articles.Update(ctx, helpcenters.ArticleInput{
		Membership: m, ID: a.ID, Title: &was.Title, Body: &was.Body, LockVersion: a.LockVersion,
	})
	return err == nil, err
}

/*
restoreProcedure puts a procedure's working copy back while it still
holds what Operator wrote.
*/
func (e *Engine) restoreProcedure(ctx context.Context, m authz.Membership, op Operation) (bool, error) {
	p, err := e.Services.Procedures.Show(ctx, m, *op.Procedure)
	if err != nil {
		return false, err
	}
	if !sameJSON(p.Draft, op.After) {
		return false, nil
	}
	_, err = e.Services.Procedures.Save(ctx, procedures.SaveInput{
		Membership: m, ID: p.ID, LockVersion: p.LockVersion, Definition: op.Before,
	})
	return err == nil, err
}

/*
sameJSON compares two JSON documents, ignoring spacing.
*/
func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}
