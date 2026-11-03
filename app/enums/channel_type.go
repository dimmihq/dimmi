/*
|--------------------------------------------------------------------------
| Channel Type
|--------------------------------------------------------------------------
|
| A channel type names where a connection's conversations come from. The
| set grows with the product, one channel at a time, so the database
| only checks a value's shape and each channel names its own type here.
|
*/

package enums

/*
ChannelType is the kind of source a channel connection is.
*/
type ChannelType string

/*
The channel types the inbox itself relies on. ChannelTypeAPI is the
connection every inbox has for conversations created through the API.
*/
const (
	ChannelTypeAPI ChannelType = "api"
)
