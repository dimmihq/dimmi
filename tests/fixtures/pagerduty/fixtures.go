/*
|--------------------------------------------------------------------------
| PagerDuty Fixtures
|--------------------------------------------------------------------------
|
| PagerDuty's API answer for a REST key and the webhook v3 deliveries of
| an incident triggered, acknowledged and resolved. No test ever reaches
| PagerDuty.
|
*/

package pagerdutyfixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
