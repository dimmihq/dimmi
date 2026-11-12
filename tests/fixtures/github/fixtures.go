/*
|--------------------------------------------------------------------------
| GitHub Issues Fixtures
|--------------------------------------------------------------------------
|
| GitHub's REST API answers for an account and issue acme/app#88, and the
| deliveries a repository webhook sends when it closes and when a
| deployment succeeds. No test ever reaches GitHub.
|
*/

package githubfixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
