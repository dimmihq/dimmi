/*
|--------------------------------------------------------------------------
| Client
|--------------------------------------------------------------------------
|
| A sign-in is judged by where it comes from as much as by what it
| proves: the address, the browser, and the device cookie a browser
| keeps for a year. The device cookie holds a random token; only its
| SHA-256 is ever stored, as the device id, so a stolen table never
| yields a cookie. The id joins a browser's sessions, lets a known
| device past a lock aimed at strangers, and never leaves the server.
|
*/

package auth

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http/useragent"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
deviceTokenLength is the length of a device cookie token: 43 letters
and digits carry more than 250 bits of randomness.
*/
const deviceTokenLength = 43

/*
userAgentLimit is how much of a User-Agent header is kept.
*/
const userAgentLimit = 512

/*
Client is the browser or program a request came from. DeviceToken is
the value of its device cookie, empty when it sent none.
*/
type Client struct {
	IP          string
	UserAgent   string
	DeviceToken string
}

/*
DeviceID returns the id of the client's device, or "" when it carries
no device cookie.
*/
func (c Client) DeviceID() string {
	if c.DeviceToken == "" {
		return ""
	}

	return hashing.SHA256Hex([]byte(c.DeviceToken))
}

/*
TruncatedUserAgent returns the User-Agent as stored, cut to a length
no header can blow up.
*/
func (c Client) TruncatedUserAgent() string {
	return str.Limit(c.UserAgent, userAgentLimit)
}

/*
NewDeviceToken returns a fresh random device cookie token.
*/
func NewDeviceToken() string {
	return str.Random(deviceTokenLength)
}

/*
Devices names devices for people, such as "Chrome on MacOS", from their
User-Agent header.
*/
type Devices struct {
	parser *useragent.Parser
}

/*
NewDevices returns the device namer. Build one at boot and share it.
*/
func NewDevices() *Devices {
	return &Devices{parser: useragent.New()}
}

/*
Label returns a short name for the device, or "" when the header says
too little.
*/
func (d *Devices) Label(userAgent string) string {
	agent := d.parser.Parse(userAgent)
	known := slices.DeleteFunc([]string{agent.Browser, agent.OS}, func(part string) bool { return part == "" })

	return strings.Join(known, " on ")
}
