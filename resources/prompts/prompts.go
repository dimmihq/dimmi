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

/*
Tasks is the whole tasks/ tree: the prompt of every job a model does
besides answering as an agent, such as judging, summarizing, sorting
and playing a simulated customer, one file per job.
*/
//
//go:embed all:tasks
var Tasks embed.FS
