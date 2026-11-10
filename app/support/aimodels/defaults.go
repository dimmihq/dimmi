/*
|--------------------------------------------------------------------------
| Default Models
|--------------------------------------------------------------------------
|
| A workspace on the built-in provider starts answering without choosing
| anything, so a fresh agent needs a model picked for it. The pick is the
| cheapest model that answers well among the families the install can
| actually serve, never one nothing upstream would answer.
|
*/

package aimodels

import "github.com/dimmihq/dimmi/app/enums"

/*
DefaultAnswer is the model a fresh agent answers with on the built-in
provider: GPT-6 Luna when the install has an OpenAI upstream, Claude
Haiku 5.5 when it has only Anthropic, GLM 5.3 Flash when it has only
open-weight models, and empty when it has none.
*/
func DefaultAnswer(configured func(enums.AIModelFamily) bool) string {
	for _, id := range []string{"gpt-6-luna", "claude-haiku-5-5", "glm-5.3-flash"} {
		if e := Find(Managed(), id); e != nil && configured(e.Family) {
			return id
		}
	}
	return ""
}
