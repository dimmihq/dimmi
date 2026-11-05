/*
|--------------------------------------------------------------------------
| Message Eraser
|--------------------------------------------------------------------------
|
| Deleting a message must reach every copy the product keeps of it: the
| raw email it arrived in, a prompt an assistant saw, a stored delivery
| body. Each part that keeps a copy registers an eraser, and deleting a
| message runs every one until each has finished.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
MessageEraser removes the copies one part of the product keeps of a
deleted message.
*/
type MessageEraser interface {
	/*
		EraseMessage removes every copy of the message the part keeps.
		It runs again after an error, so it must be safe to repeat and
		succeed once nothing is left.
	*/
	EraseMessage(ctx context.Context, workspaceID, messageID ids.UUID) error
}
