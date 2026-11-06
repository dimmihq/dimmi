/*
|--------------------------------------------------------------------------
| AI Model Role
|--------------------------------------------------------------------------
|
| The agent uses more than one model: a strong one writes the answers,
| a cheap one classifies, and an optional spare covers outages. Each
| role binds a connection and a model independently.
|
*/

package enums

/*
AIModelRole names a job a model does inside the pipeline.
*/
type AIModelRole string

/*
The model roles.
*/
const (
	AIModelRoleAnswer     AIModelRole = "answer"
	AIModelRoleClassifier AIModelRole = "classifier"
	AIModelRoleFallback   AIModelRole = "fallback"
)
