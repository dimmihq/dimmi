/*
|--------------------------------------------------------------------------
| Operator Changeset Controller
|--------------------------------------------------------------------------
|
| The changes Operator proposed, listed for review and decided one at a
| time, and what a teammate asked it to remember, which they may read and
| make it forget.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorChangesetsOutput is a list of changesets.
*/
type OperatorChangesetsOutput struct {
	Body fhttp.Collection[resources.OperatorChangeset]
}

/*
OperatorChangesetOutput is one changeset as it now stands.
*/
type OperatorChangesetOutput struct {
	Body fhttp.Resource[resources.OperatorChangeset]
}

/*
OperatorMemoriesOutput is the caller's memories.
*/
type OperatorMemoriesOutput struct {
	Body fhttp.Collection[resources.OperatorMemory]
}

/*
OperatorChangesetController serves changesets and memories.
*/
type OperatorChangesetController struct {
	Changesets *operatorchat.Changesets
	Memories   *operatorchat.Memories
	IDs        *ids.Registry
}

/*
Index lists the changesets the caller may review, of
filter[conversation] or filter[status] when sent.
*/
func (c *OperatorChangesetController) Index(ctx context.Context, _ *requests.NoInput,
) (*OperatorChangesetsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	query := r.URL.Query()
	var f operatorchat.ChangesetFilter
	if named := query.Get("filter[conversation]"); named != "" {
		id, err := c.decode(models.PrefixConversation, named)
		if err != nil {
			return nil, err
		}
		f.Conversation = &id
	}
	if status := query.Get("filter[status]"); status != "" {
		f.Status = &status
	}
	rows, err := c.Changesets.List(ctx, membership(ctx), f)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.OperatorChangeset]{Items: resources.NewOperatorChangesets(rows), Limit: 100}
	return &OperatorChangesetsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Apply applies a changeset.
*/
func (c *OperatorChangesetController) Apply(ctx context.Context, in *requests.OperatorChangesetRequest,
) (*OperatorChangesetOutput, error) {
	return c.decide(ctx, in.Changeset, c.Changesets.Apply)
}

/*
Decline turns a changeset down.
*/
func (c *OperatorChangesetController) Decline(ctx context.Context, in *requests.OperatorChangesetRequest,
) (*OperatorChangesetOutput, error) {
	return c.decide(ctx, in.Changeset, c.Changesets.Decline)
}

/*
Revert undoes an applied changeset.
*/
func (c *OperatorChangesetController) Revert(ctx context.Context, in *requests.OperatorChangesetRequest,
) (*OperatorChangesetOutput, error) {
	return c.decide(ctx, in.Changeset, c.Changesets.Revert)
}

/*
ListMemories lists the caller's memories.
*/
func (c *OperatorChangesetController) ListMemories(ctx context.Context, _ *requests.NoInput,
) (*OperatorMemoriesOutput, error) {
	rows, err := c.Memories.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.OperatorMemory]{Items: resources.NewOperatorMemories(rows), Limit: 200}
	return &OperatorMemoriesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Forget deletes one of the caller's memories.
*/
func (c *OperatorChangesetController) Forget(ctx context.Context, in *requests.OperatorMemoryRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixOperatorMemory, in.Memory)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Memories.Delete(ctx, membership(ctx), id)
}

/*
decide runs one decision on a changeset.
*/
func (c *OperatorChangesetController) decide(ctx context.Context, public string,
	fn func(context.Context, authz.Membership, ids.UUID) (models.OperatorChangeset, error),
) (*OperatorChangesetOutput, error) {
	id, err := c.decode(models.PrefixOperatorChangeset, public)
	if err != nil {
		return nil, err
	}
	row, err := fn(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &OperatorChangesetOutput{Body: fhttp.NewResource(resources.NewOperatorChangeset(row))}, nil
}

/*
decode reads a public id; anything else is not found.
*/
func (c *OperatorChangesetController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
