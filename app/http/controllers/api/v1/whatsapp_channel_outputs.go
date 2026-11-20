/*
|--------------------------------------------------------------------------
| WhatsApp Channel Outputs
|--------------------------------------------------------------------------
|
| What the WhatsApp settings endpoints answer with: a number, the
| numbers, what Meta's popup needs, an account read with a token, and
| a number's templates.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
WhatsAppChannelOutput is one WhatsApp connection.
*/
type WhatsAppChannelOutput struct {
	Body fhttp.Resource[resources.WhatsAppChannel]
}

/*
WhatsAppChannelsOutput is the workspace's WhatsApp connections.
*/
type WhatsAppChannelsOutput struct {
	Body fhttp.Collection[resources.WhatsAppChannel]
}

/*
WhatsAppSignupConfigOutput is what Meta's popup needs to open, and
whether this server can open it.
*/
type WhatsAppSignupConfigOutput struct {
	Body struct {
		Data resources.WhatsAppSignupConfig `json:"data"`
	}
}

/*
WhatsAppAccountOutput is an account read with a token.
*/
type WhatsAppAccountOutput struct {
	Body fhttp.Resource[resources.WhatsAppAccount]
}

/*
WhatsAppTemplatesOutput is a connection's templates.
*/
type WhatsAppTemplatesOutput struct {
	Body fhttp.Collection[resources.WhatsAppTemplate]
}
