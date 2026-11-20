/*
|--------------------------------------------------------------------------
| Messenger And Instagram Outputs
|--------------------------------------------------------------------------
|
| What the two channels' endpoints answer with.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
SocialChannelOutput is one connection.
*/
type SocialChannelOutput struct {
	Body fhttp.Resource[resources.SocialChannel]
}

/*
SocialChannelsOutput is the workspace's connections of a platform.
*/
type SocialChannelsOutput struct {
	Body fhttp.Collection[resources.SocialChannel]
}

/*
SocialSignupConfigOutput is what Meta's popup opens with.
*/
type SocialSignupConfigOutput struct {
	Body fhttp.Resource[resources.SocialSignupConfig]
}

/*
SocialGrantOutput is a sign-in's accounts and the grant to connect one.
*/
type SocialGrantOutput struct {
	Body fhttp.Resource[resources.SocialGrant]
}

/*
SocialConversationOutput is where a conversation stands on its platform.
*/
type SocialConversationOutput struct {
	Body fhttp.Resource[resources.SocialConversation]
}
