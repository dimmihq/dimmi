/*
|--------------------------------------------------------------------------
| Operator Skill And Knowledge Requests
|--------------------------------------------------------------------------
|
| The skill an agent reads by name, and the words it searches the
| knowledge for.
|
*/

package requests

/*
OperatorSkillRequest names one skill.
*/
type OperatorSkillRequest struct {
	Skill string `path:"skill"`
}

/*
SearchOperatorKnowledgeRequest is the query of GET
/operator/knowledge/search.
*/
type SearchOperatorKnowledgeRequest struct {
	SearchText string `query:"search_text" required:"true" minLength:"1" maxLength:"500"`
}
