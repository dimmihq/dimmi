/*
|--------------------------------------------------------------------------
| incident.io Fixtures
|--------------------------------------------------------------------------
|
| incident.io's API answers for an API key, its severities, and the
| deliveries it sends when an incident opens, gets an update or closes.
| No test ever reaches incident.io.
|
*/

package incidentiofixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
