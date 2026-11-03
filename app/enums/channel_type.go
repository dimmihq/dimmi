/*
|--------------------------------------------------------------------------
| Channel Type
|--------------------------------------------------------------------------
|
| Where a conversation's messages travel: the messenger on the
| customer's site, email, a chat app, or the API. The type is a
| registry value in the database, so a new channel is new code and
| never a migration; these are the types the product ships.
|
*/

package enums

/*
ChannelType is the kind of a channel connection.
*/
type ChannelType string

/*
The channel types.
*/
const (
	ChannelTypeWidget ChannelType = "widget"
	ChannelTypeEmail  ChannelType = "email"
	ChannelTypeAPI    ChannelType = "api"
)
