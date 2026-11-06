/*
|--------------------------------------------------------------------------
| Agent Prompts
|--------------------------------------------------------------------------
|
| The agent's prompt files ship inside the binary, version by
| version, so a run always answers with the text it traced and an old
| binary never reads prompts written for a newer one.
|
*/

package prompts

import "embed"

/*
Files is the whole agent/ tree.
*/
//
//go:embed all:agent
var Files embed.FS
