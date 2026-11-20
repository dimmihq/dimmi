/*
|--------------------------------------------------------------------------
| Rootly Fixtures
|--------------------------------------------------------------------------
|
| Rootly's API answers for an API key, an incident, a search and the
| services, and deliveries recorded from a live endpoint: an incident
| mitigated, a note posted on its timeline and the events Rootly writes
| there itself. No test ever reaches Rootly.
|
*/

package rootlyfixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
