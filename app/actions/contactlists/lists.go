/*
|--------------------------------------------------------------------------
| Contact Lists
|--------------------------------------------------------------------------
|
| A team often has a list of customers from elsewhere: the people at an
| event, the accounts a salesperson owns. Uploading it as a CSV puts one
| tag on every customer it names, by email or by user id, and a tag is
| what audiences, filters and workflows already read. Only customers the
| workspace already has are tagged; the rest are counted and a few shown,
| never created. A customer who already holds 50 tags is skipped.
|
*/

package contactlists

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"slices"
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
Limits of one list.
*/
const (
	MaxRows   = 50_000
	MaxBytes  = 5 << 20
	chunk     = 1000
	shownMiss = 20
)

/*
The failures of a list.
*/
var (
	ErrUnreadable = fault.New(fault.Invalid, "list_unreadable",
		"Upload a CSV file with a header row, 50,000 rows and 5 MB at most.").OnField("file")
	ErrNoColumn = fault.New(fault.Invalid, "list_column_missing",
		"Name a column email, external_id or user_id in the header row.").OnField("file")
	ErrTagName = fault.New(fault.Invalid, "tag_name_invalid", "Name the tag, 1 to 50 characters.").OnField("tag")
)

/*
Lists tags customers from uploaded lists.
*/
type Lists struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
ImportInput is a list to tag customers from.
*/
type ImportInput struct {
	Membership authz.Membership
	TagName    string
	File       io.Reader
}

/*
Report is what a list did: the tag, how many values it held, how many
named a customer, how many customers it newly tagged, and some values
that named nobody.
*/
type Report struct {
	Tag       models.Tag
	Rows      int
	Matched   int
	Tagged    int64
	Unmatched int
	Missing   []string
}

/*
Import tags the customers the list names.
*/
func (l *Lists) Import(ctx context.Context, in ImportInput) (Report, error) {
	m := in.Membership
	if !m.Can("contacts:write") {
		return Report{}, exceptions.ErrForbidden
	}

	name := strings.TrimSpace(in.TagName)
	if name == "" || len([]rune(name)) > 50 {
		return Report{}, ErrTagName
	}

	column, values, err := read(in.File)
	if err != nil {
		return Report{}, err
	}

	var out Report
	err = l.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), m.Workspace.ID, l.Clock.Now()
		tag, err := EnsureTag(ctx, q, Tagging{WorkspaceID: ws, Now: now}, name)
		if err != nil {
			return err
		}

		found, missing, err := match(ctx, q, lookup{ws: ws, column: column, values: values})
		if err != nil {
			return err
		}

		tagged, err := Tag(ctx, q, Tagging{
			WorkspaceID: ws, Tag: tag.ID, Member: m.Member.ID, Contacts: found, Now: now,
		})
		out = Report{
			Tag: tag, Rows: len(values), Matched: len(found), Tagged: tagged, Unmatched: len(missing),
			Missing: missing[:min(len(missing), shownMiss)],
		}

		return err
	})

	return out, err
}

/*
read returns the column the list names customers by and its values,
each once.
*/
func read(file io.Reader) (string, []string, error) {
	records, err := records(file)
	if err != nil {
		return "", nil, err
	}

	at, column := columnOf(records[0])
	if at < 0 {
		return "", nil, ErrNoColumn
	}

	var values []string
	for _, r := range records[1:] {
		if v := valueAt(r, at, column); v != "" && !slices.Contains(values, v) {
			values = append(values, v)
		}
	}

	return column, values, nil
}

/*
records reads the file's rows, the header first.
*/
func records(file io.Reader) ([][]string, error) {
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil || len(data) > MaxBytes {
		return nil, ErrUnreadable
	}

	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\ufeff"))))
	reader.FieldsPerRecord = -1
	out, err := reader.ReadAll()
	if err != nil || len(out) < 1 || len(out) > MaxRows+1 {
		return nil, ErrUnreadable
	}

	return out, nil
}

/*
columnOf finds the column customers are named by, -1 when none.
*/
func columnOf(header []string) (int, string) {
	for i, cell := range header {
		h := strings.ToLower(strings.TrimSpace(cell))
		if h == "email" || h == "external_id" || h == "user_id" {
			return i, h
		}
	}

	return -1, ""
}

/*
valueAt is a row's value in the column, emails in lower case.
*/
func valueAt(r []string, at int, column string) string {
	if at >= len(r) {
		return ""
	}

	v := strings.TrimSpace(r[at])
	if column == "email" {
		return strings.ToLower(v)
	}

	return v
}

/*
match finds the customers the values name, returning the values that
named nobody.
*/
func match(ctx context.Context, q *models.Queries, all lookup) ([]ids.UUID, []string, error) {
	ws, column, values := all.ws, all.column, all.values
	named := map[string]ids.UUID{}
	for start := 0; start < len(values); start += chunk {
		part := values[start:min(start+chunk, len(values))]
		if err := lookUp(ctx, q, lookup{ws: ws, column: column, values: part}, named); err != nil {
			return nil, nil, err
		}
	}

	var found []ids.UUID
	var missing []string
	for _, v := range values {
		if id, ok := named[v]; ok {
			found = append(found, id)
		} else {
			missing = append(missing, v)
		}
	}

	return found, missing, nil
}

/*
lookup is one page of values to find customers by.
*/
type lookup struct {
	ws     ids.UUID
	column string
	values []string
}

/*
lookUp records which customer each value names.
*/
func lookUp(ctx context.Context, q *models.Queries, l lookup, named map[string]ids.UUID) error {
	if l.column == "email" {
		rows, err := q.ContactsByEmails(ctx, models.ContactsByEmailsParams{WorkspaceID: l.ws, Emails: l.values})
		for _, r := range rows {
			named[strings.ToLower(deref(r.Email))] = r.ID
		}

		return wrap("match emails", err)
	}

	rows, err := q.ContactsByExternalIDs(ctx, models.ContactsByExternalIDsParams{
		WorkspaceID: l.ws, ExternalIds: l.values,
	})
	for _, r := range rows {
		named[deref(r.ExternalID)] = r.ID
	}

	return wrap("match user ids", err)
}

func wrap(what string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}

	return nil
}

/*
Tagging puts one tag on many customers.
*/
type Tagging struct {
	WorkspaceID ids.UUID
	Tag         ids.UUID
	Member      ids.UUID
	Contacts    []ids.UUID
	Now         time.Time
}

/*
Tag puts the tag on the customers in pages and counts those newly
tagged.
*/
func Tag(ctx context.Context, q *models.Queries, t Tagging) (int64, error) {
	var tagged int64
	for start := 0; start < len(t.Contacts); start += chunk {
		n, err := q.TagContactsInBulk(ctx, models.TagContactsInBulkParams{
			WorkspaceID: t.WorkspaceID, TagID: t.Tag, CreatedByType: "member", CreatedByID: &t.Member, Now: t.Now,
			ContactIds: t.Contacts[start:min(start+chunk, len(t.Contacts))],
		})
		if err != nil {
			return tagged, fmt.Errorf("tag contacts: %w", err)
		}

		tagged += n
	}

	return tagged, nil
}

/*
EnsureTag returns the workspace's tag with the name, making it when
there is none. Only the tagging's workspace and time are read.
*/
func EnsureTag(ctx context.Context, q *models.Queries, t Tagging, name string) (models.Tag, error) {
	tags, err := q.EnsureTags(ctx, models.EnsureTagsParams{
		WorkspaceID: t.WorkspaceID, Names: []string{name}, Now: t.Now,
	})
	if err != nil {
		return models.Tag{}, fmt.Errorf("ensure tag: %w", err)
	}

	if len(tags) == 0 {
		return models.Tag{}, fmt.Errorf("ensure tag %q: made at the same moment elsewhere", name)
	}

	return models.Tag(tags[0]), nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
