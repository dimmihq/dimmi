/*
|--------------------------------------------------------------------------
| Channel Type
|--------------------------------------------------------------------------
|
| A channel type names where a connection's conversations come from:
| the messenger on the customer's site, email, a chat app, or the API.
| The set grows with the product, one channel at a time, so the database
| only checks a value's shape and each channel names its own type here.
|
*/

package enums

/*
ChannelType is the kind of source a channel connection is.
*/
type ChannelType string

/*
The channel types the product ships. ChannelTypeAPI is the connection
every inbox has for conversations created through the API.
*/
const (
	ChannelTypeWidget ChannelType = "widget"
	ChannelTypeEmail  ChannelType = "email"
	ChannelTypeAPI    ChannelType = "api"
)
