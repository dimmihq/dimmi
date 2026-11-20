/*
|--------------------------------------------------------------------------
| Customer Notes
|--------------------------------------------------------------------------
|
| What the team knows about a customer or a company, kept as separate
| notes: each names the teammate who wrote it and when, and one of a
| record's notes can be pinned so it shows first. A teammate rewrites or removes their
| own notes; a note with no author, such as the one carried over from the
| single notes text records held before, is anyone's to tidy. Reading
| takes contacts:read and writing contacts:write.
|
*/

package contacts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxNoteRunes caps a note.
*/
const maxNoteRunes = 10000

/*
Errors notes return.
*/
var (
	ErrNoteBody = fault.New(fault.Invalid, "contact_note_body_invalid",
		"Write a note of up to 10,000 characters.").OnField("body")
	ErrNoteNotYours = fault.New(fault.Forbidden, "contact_note_not_yours",
		"Only the teammate who wrote a note can change or delete it.")
)

/*
NoteSubject is who a note is about: a customer or a company, one of the
two.
*/
type NoteSubject struct {
	ContactID *ids.UUID
	CompanyID *ids.UUID
}

/*
AboutContact is the subject of a customer's notes.
*/
func AboutContact(id ids.UUID) NoteSubject {
	return NoteSubject{ContactID: &id}
}

/*
AboutCompany is the subject of a company's notes.
*/
func AboutCompany(id ids.UUID) NoteSubject {
	return NoteSubject{CompanyID: &id}
}

/*
Note is one note about a customer or a company with who wrote it.
*/
type Note struct {
	ID         ids.UUID
	ContactID  *ids.UUID
	CompanyID  *ids.UUID
	AuthorID   *ids.UUID
	AuthorName string
	Body       string
	PinnedAt   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

/*
NoteChange is what a teammate changes about a note: its words, whether
it is pinned, or both.
*/
type NoteChange struct {
	Body   *string
	Pinned *bool
}

/*
Notes reads and writes the notes about customers.
*/
type Notes struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
List returns the notes about the customer or company, the pinned one
first, then newest first.
*/
func (a *Notes) List(ctx context.Context, m authz.Membership, about NoteSubject) ([]Note, error) {
	if !m.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Note

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), m.Workspace.ID

		about, err := subject(ctx, q, ws, about)
		if err != nil {
			return err
		}

		rows, err := q.ListCustomerNotes(ctx, models.ListCustomerNotesParams{
			WorkspaceID: ws, ContactID: about.ContactID, CompanyID: about.CompanyID,
		})
		if err != nil {
			return fmt.Errorf("list notes: %w", err)
		}

		out = make([]Note, len(rows))
		for i, r := range rows {
			out[i] = Note{
				ID: r.ID, ContactID: r.ContactID, CompanyID: r.CompanyID, AuthorID: r.AuthorMemberID,
				AuthorName: r.AuthorName, Body: r.Body, PinnedAt: r.PinnedAt, CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			}
		}

		return nil
	})

	return out, err
}

/*
Create adds a note about the customer or company, written by the
member.
*/
func (a *Notes) Create(ctx context.Context, m authz.Membership, about NoteSubject, body string) (Note, error) {
	if !m.Can("contacts:write") {
		return Note{}, exceptions.ErrForbidden
	}

	body, err := noteBody(body)
	if err != nil {
		return Note{}, err
	}

	var out Note

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), m.Workspace.ID

		about, err := subject(ctx, q, ws, about)
		if err != nil {
			return err
		}

		id, err := q.CreateCustomerNote(ctx, models.CreateCustomerNoteParams{
			WorkspaceID: ws, ContactID: about.ContactID, CompanyID: about.CompanyID, AuthorMemberID: authorOf(m),
			Body: body, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create note: %w", err)
		}

		out, err = findNote(ctx, q, ws, id)

		return err
	})

	return out, err
}

/*
Update rewrites a note, which only its author may, or pins and unpins
it, which anyone who writes notes may. Pinning one takes the pin off
the customer's others.
*/
func (a *Notes) Update(ctx context.Context, m authz.Membership, id ids.UUID, change NoteChange) (Note, error) {
	if !m.Can("contacts:write") {
		return Note{}, exceptions.ErrForbidden
	}

	var out Note

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), m.Workspace.ID

		note, err := findNote(ctx, q, ws, id)
		if err != nil {
			return err
		}

		w := noteWriter{q: q, m: m, now: a.Clock.Now()}

		if err := w.rewrite(ctx, note, change.Body); err != nil {
			return err
		}

		if err := w.pin(ctx, note, change.Pinned); err != nil {
			return err
		}

		out, err = findNote(ctx, q, ws, id)

		return err
	})

	return out, err
}

/*
noteWriter changes a note for one member at one moment.
*/
type noteWriter struct {
	q   *models.Queries
	m   authz.Membership
	now time.Time
}

/*
rewrite changes the note's words when asked to.
*/
func (w noteWriter) rewrite(ctx context.Context, note Note, body *string) error {
	if body == nil {
		return nil
	}

	if !mayChange(w.m, note) {
		return ErrNoteNotYours
	}

	words, err := noteBody(*body)
	if err != nil {
		return err
	}

	err = w.q.UpdateCustomerNoteBody(ctx, models.UpdateCustomerNoteBodyParams{
		Body: words, Now: w.now, WorkspaceID: w.m.Workspace.ID, ID: note.ID,
	})
	if err != nil {
		return fmt.Errorf("update contact note: %w", err)
	}

	return nil
}

/*
pin pins or unpins the note when asked to.
*/
func (w noteWriter) pin(ctx context.Context, note Note, pinned *bool) error {
	if pinned == nil {
		return nil
	}

	ws := w.m.Workspace.ID

	err := w.q.UnpinCustomerNotes(ctx, models.UnpinCustomerNotesParams{
		WorkspaceID: ws, ContactID: note.ContactID, CompanyID: note.CompanyID,
	})
	if err != nil {
		return fmt.Errorf("unpin notes: %w", err)
	}

	if !*pinned {
		return nil
	}

	err = w.q.PinCustomerNote(ctx, models.PinCustomerNoteParams{Now: w.now, WorkspaceID: ws, ID: note.ID})
	if err != nil {
		return fmt.Errorf("pin contact note: %w", err)
	}

	return nil
}

/*
Delete removes a note, which only its author may.
*/
func (a *Notes) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("contacts:write") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), m.Workspace.ID

		note, err := findNote(ctx, q, ws, id)
		if err != nil {
			return err
		}

		if !mayChange(m, note) {
			return ErrNoteNotYours
		}

		if err := q.DeleteCustomerNote(ctx, models.DeleteCustomerNoteParams{WorkspaceID: ws, ID: id}); err != nil {
			return fmt.Errorf("delete contact note: %w", err)
		}

		return nil
	})
}

/*
findNote reads one note of the workspace.
*/
func findNote(ctx context.Context, q *models.Queries, ws, id ids.UUID) (Note, error) {
	r, err := q.FindCustomerNote(ctx, models.FindCustomerNoteParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return Note{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Note{}, fmt.Errorf("find contact note: %w", err)
	}

	return Note{
		ID: r.ID, ContactID: r.ContactID, CompanyID: r.CompanyID, AuthorID: r.AuthorMemberID,
		AuthorName: r.AuthorName, Body: r.Body, PinnedAt: r.PinnedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}

/*
subject checks the customer or company exists in the workspace and
returns it as it stands: a merged customer as the one that survived.
*/
func subject(ctx context.Context, q *models.Queries, ws ids.UUID, about NoteSubject) (NoteSubject, error) {
	if about.ContactID != nil {
		c, err := Survivor(ctx, q, ws, *about.ContactID)
		if err != nil {
			return NoteSubject{}, err
		}

		return AboutContact(c.ID), nil
	}

	if about.CompanyID == nil {
		return NoteSubject{}, exceptions.ErrNotFound
	}

	company, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: ws, ID: *about.CompanyID})
	if errors.Is(err, database.ErrNotFound) || (err == nil && company.DeletedAt != nil) {
		return NoteSubject{}, exceptions.ErrNotFound
	}

	if err != nil {
		return NoteSubject{}, fmt.Errorf("find company: %w", err)
	}

	return AboutCompany(company.ID), nil
}

/*
noteBody trims a note's words and refuses none or too many.
*/
func noteBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if n := len([]rune(body)); n < 1 || n > maxNoteRunes {
		return "", ErrNoteBody
	}

	return body, nil
}

/*
authorOf is the member writing, or nobody for a caller that is not a
teammate, such as an API token.
*/
func authorOf(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}

	id := m.Member.ID

	return &id
}

/*
mayChange reports whether the member may rewrite or remove the note:
its author, or anyone when it has none.
*/
func mayChange(m authz.Membership, note Note) bool {
	return note.AuthorID == nil || *note.AuthorID == m.Member.ID
}
