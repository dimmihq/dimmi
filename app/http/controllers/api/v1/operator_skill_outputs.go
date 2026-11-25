/*
|--------------------------------------------------------------------------
| Operator Skill And Knowledge Outputs
|--------------------------------------------------------------------------
|
| What the routes for agents doing Operator's work answer with about
| skills and the knowledge.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
OperatorSkillsOutput is the skills.
*/
type OperatorSkillsOutput struct {
	Body fhttp.Collection[resources.OperatorSkill]
}

/*
OperatorSkillOutput is one skill with its steps.
*/
type OperatorSkillOutput struct {
	Body fhttp.Resource[resources.OperatorSkill]
}

/*
OperatorKnowledgePassagesOutput is the passages a search found.
*/
type OperatorKnowledgePassagesOutput struct {
	Body fhttp.Collection[resources.OperatorKnowledgePassage]
}
