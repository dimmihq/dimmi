/*
|--------------------------------------------------------------------------
| Audit Log Controller
|--------------------------------------------------------------------------
|
| Serves the audit log of the workspace in play. Filters name members,
| tokens and subjects by their public ids, and times as RFC 3339; a
| filter value that cannot name anything is refused rather than
| ignored, so a typo never reads as an empty log.
|
*/

package v1

import (
	"context"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AuditLogsOutput is a page of the audit log.
*/
type AuditLogsOutput struct {
	Body fhttp.Collection[resources.AuditLog]
}

/*
AuditLogOutput is one entry of the audit log.
*/
type AuditLogOutput struct {
	Body fhttp.Resource[resources.AuditLog]
}

/*
AuditLogController serves the workspace's audit log.
*/
type AuditLogController struct {
	List    *audit.ListAuditLogs
	Find    *audit.ShowAuditLog
	IDs     *ids.Registry
	Cursors *pagination.Cursors
}

/*
auditFilters are the filters the audit log accepts.
*/
func auditFilters() []string {
	return []string{"event", "actor", "via", "subject", "created_after", "created_before"}
}

/*
Index lists a page of the audit log, newest first or, with
sort=created_at, oldest first.
*/
func (c *AuditLogController) Index(ctx context.Context, _ *requests.NoInput) (*AuditLogsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	allow := pagination.Allow{Filters: auditFilters(), Sorts: []string{"created_at"}}

	query, err := pagination.Parse(r.URL.Query(), allow)
	if err != nil {
		return nil, err
	}

	in, err := c.listInput(ctx, query)
	if err != nil {
		return nil, err
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.AuditLog]{Items: resources.NewAuditLogs(page.Entries), Limit: in.Limit}
	if page.More {
		last := page.Entries[len(page.Entries)-1]
		if out.NextCursor, err = c.Cursors.Encode(audit.Position{At: last.CreatedAt, ID: last.ID}); err != nil {
			return nil, err
		}
	}

	return &AuditLogsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Show answers with one entry of the audit log.
*/
func (c *AuditLogController) Show(
	ctx context.Context, in *requests.ShowAuditLogRequest,
) (*AuditLogOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAuditLog), in.AuditLog)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	entry, err := c.Find.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &AuditLogOutput{Body: fhttp.NewResource(resources.NewAuditLog(entry))}, nil
}

/*
listInput turns the list query into the action's input.
*/
func (c *AuditLogController) listInput(ctx context.Context, query pagination.Query) (audit.ListAuditLogsInput, error) {
	in := audit.ListAuditLogsInput{Membership: membership(ctx), Limit: query.Limit}

	for _, sort := range query.Sorts {
		in.Ascending = !sort.Desc
	}

	filter, err := c.filter(query.Filters)
	if err != nil {
		return in, err
	}

	in.Filter = filter

	if query.Cursor != "" {
		var from audit.Position
		if err := c.Cursors.Decode(query.Cursor, &from); err != nil {
			return in, err
		}

		in.From = &from
	}

	return in, nil
}

/*
filter reads the filters of the query.
*/
func (c *AuditLogController) filter(filters map[string][]string) (audit.Filter, error) {
	var (
		f   = audit.Filter{Events: filters["event"]}
		err error
	)

	if f.ActorID, err = c.filterID(filters, "actor", models.PrefixMember); err != nil {
		return f, err
	}

	if f.SubjectID, err = c.filterID(filters, "subject", subjectPrefixes()...); err != nil {
		return f, err
	}

	if f.ViaID, err = c.filterID(filters, "via", viaPrefixes()...); err != nil {
		return f, err
	}

	if f.CreatedAfter, err = filterTime(filters, "created_after"); err != nil {
		return f, err
	}

	f.CreatedBefore, err = filterTime(filters, "created_before")

	return f, err
}

/*
filterID returns the id the filter names with any of the prefixes, nil
when the filter is absent.
*/
func (c *AuditLogController) filterID(
	filters map[string][]string, name string, prefixes ...models.Prefix,
) (*ids.UUID, error) {
	value, ok := single(filters, name)
	if !ok {
		return nil, nil
	}

	for _, prefix := range prefixes {
		if id, err := c.IDs.Decode(string(prefix), value); err == nil {
			return &id, nil
		}
	}

	return nil, pagination.ErrInvalidFilter.With("parameter", name)
}

/*
filterTime returns the time the filter names, nil when the filter is
absent. A query string decodes the plus of an offset such as +01:00 to
a space, which is read back as a plus.
*/
func filterTime(filters map[string][]string, name string) (*time.Time, error) {
	value, ok := single(filters, name)
	if !ok {
		return nil, nil
	}

	at, err := time.Parse(time.RFC3339, strings.ReplaceAll(value, " ", "+"))
	if err != nil {
		return nil, pagination.ErrInvalidFilter.With("parameter", name)
	}

	return &at, nil
}

/*
single returns the one value of a filter: a time or an id is single, so
a list keeps only its first value.
*/
func single(filters map[string][]string, name string) (string, bool) {
	values := filters[name]
	if len(values) == 0 {
		return "", false
	}

	return values[0], true
}

/*
subjectPrefixes lists the kinds of subject a filter may name.
*/
func subjectPrefixes() []models.Prefix {
	return []models.Prefix{
		models.PrefixMember, models.PrefixWorkspace, models.PrefixTeam, models.PrefixInvitation,
		models.PrefixPersonalToken,
	}
}

/*
viaPrefixes lists the credentials a filter may name: personal tokens,
OAuth clients and first-party app grants.
*/
func viaPrefixes() []models.Prefix {
	return []models.Prefix{models.PrefixPersonalToken, models.PrefixOAuthClient, models.PrefixOAuthGrant}
}
