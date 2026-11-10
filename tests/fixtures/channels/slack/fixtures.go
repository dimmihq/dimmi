/*
|--------------------------------------------------------------------------
| Slack Fixtures
|--------------------------------------------------------------------------
|
| Slack's Web API answers as Slack sends them, one file per method, for
| the Slack channel's tests: the app's own identity, users of the
| installing team and of a customer's organization, the channels the
| app is in, posted messages and reactions. A users.info answer per user
| is named after the method and the user. No test ever reaches Slack.
|
*/

package slack

import "embed"

/*
Files are the recorded answers, named after the Web API method.
*/
//go:embed *.json
var Files embed.FS
