/*
|--------------------------------------------------------------------------
| Rootly Fixtures
|--------------------------------------------------------------------------
|
| Rootly's API answer for an API key and the delivery sent when a
| critical incident opens. No test ever reaches Rootly.
|
*/

package rootlyfixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
