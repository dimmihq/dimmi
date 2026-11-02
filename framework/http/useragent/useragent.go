/*
|--------------------------------------------------------------------------
| User Agents
|--------------------------------------------------------------------------
|
| Session lists and sign-in alerts name the device a person used, such as
| "Chrome on MacOS", so they can spot one they do not recognize. The
| result is a label for people, never a security decision: any client can
| send any User-Agent. Parsing is a lookup in a prebuilt tree with no
| regular expressions, cheap enough to run on every sign-in.
|
*/

package useragent

import (
	"github.com/medama-io/go-useragent"
	"github.com/medama-io/go-useragent/agents"
)

/*
The kinds of device an Agent reports. An empty Device means the header
named none of them.
*/
const (
	Desktop = "desktop"
	Mobile  = "mobile"
	Tablet  = "tablet"
	TV      = "tv"
	Bot     = "bot"
)

/*
Agent is what a User-Agent header says about the client.

Browser and OS are display names such as "Chrome" and "MacOS", and
BrowserVersion the full version such as "120.0.0.0". Device is one of
Desktop, Mobile, Tablet, TV or Bot. Each is empty when the header does
not say.
*/
type Agent struct {
	Browser        string
	BrowserVersion string
	OS             string
	Device         string
}

/*
Parser reads User-Agent headers. It is safe for concurrent use; build one
at boot and share it.
*/
type Parser struct {
	parser *useragent.Parser
}

/*
New returns a parser loaded with the library's built-in list of agents.
*/
func New() *Parser {
	return &Parser{parser: useragent.NewParser()}
}

/*
Parse reads the header. An empty header gives an empty Agent, and a
client that names no known browser, such as a script, is reported as a
Bot.
*/
func (p *Parser) Parse(ua string) Agent {
	parsed := p.parser.Parse(ua)

	return Agent{
		Browser:        string(parsed.Browser()),
		BrowserVersion: parsed.BrowserVersion(),
		OS:             string(parsed.OS()),
		Device:         device(parsed.Device()),
	}
}

func device(d agents.Device) string {
	switch d {
	case agents.DeviceDesktop:
		return Desktop
	case agents.DeviceMobile:
		return Mobile
	case agents.DeviceTablet:
		return Tablet
	case agents.DeviceTV:
		return TV
	case agents.DeviceBot:
		return Bot
	}

	return ""
}
