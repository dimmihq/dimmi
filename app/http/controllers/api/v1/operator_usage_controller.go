/*
|--------------------------------------------------------------------------
| Operator Usage Controller
|--------------------------------------------------------------------------
|
| How Operator was used in a month, for the agent's page: what it cost,
| how often it ran and for whom. Reading it needs the same access as the
| agent's own usage.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
OperatorUsageOutput is one month of Operator.
*/
type OperatorUsageOutput struct {
	Body fhttp.Resource[resources.OperatorUsage]
}

/*
OperatorUsageController serves Operator's usage.
*/
type OperatorUsageController struct {
	Usage *operatorchat.Usage
}

/*
Show returns one month of Operator's usage.
*/
func (c *OperatorUsageController) Show(ctx context.Context, in *requests.AIUsageRequest) (*OperatorUsageOutput, error) {
	view, err := c.Usage.Handle(ctx, operatorchat.UsageInput{Membership: membership(ctx), Period: in.Period})
	if err != nil {
		return nil, err
	}
	return &OperatorUsageOutput{Body: fhttp.NewResource(resources.NewOperatorUsage(*view))}, nil
}
