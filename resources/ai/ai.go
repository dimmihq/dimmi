/*
|--------------------------------------------------------------------------
| AI Resources
|--------------------------------------------------------------------------
|
| The agent's shipped data files — today the model price table —
| compiled into the binary so a run's cost always matches the code
| that made it.
|
*/

package ai

import "embed"

/*
Files is the resource tree.
*/
//
//go:embed all:*
var Files embed.FS
