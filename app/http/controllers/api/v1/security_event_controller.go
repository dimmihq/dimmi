/*
|--------------------------------------------------------------------------
| Security Event Controller
|--------------------------------------------------------------------------
|
| Serves the caller's own security log, newest first, a page at a time.
| Only the user themselves reads it, through their browser or their own
| app; no workspace and no token can.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
SecurityEventsOutput is a page of the caller's security log.
*/
type SecurityEventsOutput struct {
	Body fhttp.Collection[resources.SecurityEvent]
}

/*
SecurityEventController serves the caller's security log.
*/
type SecurityEventController struct {
	List    *auth.ListSecurityEvents
	Cursors *pagination.Cursors
}

/*
Index lists a page of the caller's security log.
*/
func (c *SecurityEventController) Index(ctx context.Context, _ *requests.NoInput) (*SecurityEventsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{})
	if err != nil {
		return nil, err
	}

	in := auth.ListSecurityEventsInput{UserID: caller(ctx).User.ID, Limit: query.Limit}
	if query.Cursor != "" {
		var from auth.SecurityEventPosition
		if err := c.Cursors.Decode(query.Cursor, &from); err != nil {
			return nil, err
		}

		in.From = &from
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.SecurityEvent]{Items: resources.NewSecurityEvents(page.Events), Limit: in.Limit}
	if page.More {
		last := page.Events[len(page.Events)-1]
		next := auth.SecurityEventPosition{At: last.CreatedAt, ID: last.ID}
		if out.NextCursor, err = c.Cursors.Encode(next); err != nil {
			return nil, err
		}
	}

	return &SecurityEventsOutput{Body: fhttp.NewCollection(out)}, nil
}
