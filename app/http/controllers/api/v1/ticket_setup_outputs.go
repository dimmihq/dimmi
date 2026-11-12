/*
|--------------------------------------------------------------------------
| Ticket Setup Outputs
|--------------------------------------------------------------------------
|
| What the setup endpoints answer about states and fields.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
TicketFieldsOutput is a type's fields.
*/
type TicketFieldsOutput struct {
	Body fhttp.Resource[[]resources.TicketField]
}

/*
TicketFieldOutput is one field.
*/
type TicketFieldOutput struct {
	Body fhttp.Resource[resources.TicketField]
}

/*
TicketStatesOutput is every state.
*/
type TicketStatesOutput struct {
	Body fhttp.Resource[[]resources.TicketState]
}

/*
TicketStateOutput is one state.
*/
type TicketStateOutput struct {
	Body fhttp.Resource[resources.TicketState]
}
