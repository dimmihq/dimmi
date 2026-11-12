/*
|--------------------------------------------------------------------------
| Slack Event Fixtures
|--------------------------------------------------------------------------
|
| Deliveries as Slack sends them to the app, one file per kind: a
| customer's message in a shared channel, a teammate's ticket reaction
| on it, and the ticket message shortcut. Tests change the channel,
| user or timestamp they need before delivering one.
|
*/

package events

import "embed"

/*
Files are the recorded deliveries, named after what they carry.
*/
//go:embed *.json
var Files embed.FS
