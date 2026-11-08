/*
|--------------------------------------------------------------------------
| Procedure Controller
|--------------------------------------------------------------------------
|
| GET /procedures lists the workspace's procedures with their use; POST
| adds an empty draft; PATCH saves the name or working copy; POST
| .../publications makes the working copy what dimmi follows and
| DELETE .../publications takes it back; DELETE removes a procedure.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ProcedureController serves procedures.
*/
type ProcedureController struct {
	Procedures *procedures.Procedures
	IDs        *ids.Registry
}

/*
ProceduresOutput is the workspace's procedures.
*/
type ProceduresOutput struct {
	Body fhttp.Collection[resources.Procedure]
}

/*
ProcedureOutput is one procedure.
*/
type ProcedureOutput struct {
	Body fhttp.Resource[resources.Procedure]
}

/*
Index lists the procedures.
*/
func (c *ProcedureController) Index(ctx context.Context, _ *requests.NoInput) (*ProceduresOutput, error) {
	rows, err := c.Procedures.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.Procedure, len(rows))
	for i, r := range rows {
		items[i] = resources.NewProcedure(r.Procedure, r.Uses)
	}

	page := pagination.Page[resources.Procedure]{Items: items, Limit: 500}

	return &ProceduresOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one procedure.
*/
func (c *ProcedureController) Show(ctx context.Context, in *requests.ProcedureRequest) (*ProcedureOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	p, err := c.Procedures.Show(ctx, membership(ctx), id)

	return procedureOutput(p, err)
}

/*
Store adds an empty draft procedure.
*/
func (c *ProcedureController) Store(ctx context.Context, in *requests.StoreProcedureRequest) (*ProcedureOutput, error) {
	p, err := c.Procedures.Create(ctx, membership(ctx), in.Body.Name)

	return procedureOutput(p, err)
}

/*
Update saves the name or working copy.
*/
func (c *ProcedureController) Update(ctx context.Context, in *requests.UpdateProcedureRequest,
) (*ProcedureOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	p, err := c.Procedures.Save(ctx, procedures.SaveInput{
		Membership: membership(ctx), ID: id, LockVersion: in.Body.LockVersion, Name: in.Body.Name,
		Definition: in.Body.Definition,
	})

	return procedureOutput(p, err)
}

/*
Publish makes the working copy what dimmi follows.
*/
func (c *ProcedureController) Publish(ctx context.Context, in *requests.ProcedureRequest) (*ProcedureOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	p, err := c.Procedures.Publish(ctx, membership(ctx), id)

	return procedureOutput(p, err)
}

/*
Unpublish takes the procedure out of dimmi's hands.
*/
func (c *ProcedureController) Unpublish(ctx context.Context, in *requests.ProcedureRequest) (*ProcedureOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	p, err := c.Procedures.Unpublish(ctx, membership(ctx), id)

	return procedureOutput(p, err)
}

/*
Destroy removes a procedure.
*/
func (c *ProcedureController) Destroy(ctx context.Context, in *requests.ProcedureRequest) (*EmptyOutput, error) {
	id, err := c.decode(in.Procedure)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Procedures.Delete(ctx, membership(ctx), id)
}

func procedureOutput(p models.Procedure, err error) (*ProcedureOutput, error) {
	if err != nil {
		return nil, err
	}

	return &ProcedureOutput{Body: fhttp.NewResource(resources.NewProcedure(p, 0))}, nil
}

func (c *ProcedureController) decode(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixProcedure), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
