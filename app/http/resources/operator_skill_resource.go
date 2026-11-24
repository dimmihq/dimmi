/*
|--------------------------------------------------------------------------
| Operator Skill And Knowledge Resources
|--------------------------------------------------------------------------
|
| What an agent doing Operator's work reads before it starts: a skill's
| steps, and the passages of the knowledge the AI agent answers from.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorSkill is one way of doing a kind of routine work well. Steps is
left out of a list.
*/
type OperatorSkill struct {
	Object    string `json:"object"`
	SkillName string `json:"skill_name"`
	WhenToUse string `json:"when_to_use"`
	Steps     string `json:"steps,omitempty"`
}

/*
NewOperatorSkill shapes a skill with its steps.
*/
func NewOperatorSkill(s operator.Skill) OperatorSkill {
	return OperatorSkill{Object: "operator_skill", SkillName: s.Name, WhenToUse: s.Use, Steps: s.Body}
}

/*
NewOperatorSkills shapes the skills for a list, without their steps.
*/
func NewOperatorSkills(all []operator.Skill) []OperatorSkill {
	out := make([]OperatorSkill, len(all))
	for i, s := range all {
		out[i] = OperatorSkill{Object: "operator_skill", SkillName: s.Name, WhenToUse: s.Use}
	}
	return out
}

/*
OperatorKnowledgePassage is one passage of the knowledge the AI agent
answers from. ArticleID names the help article it is from, nil for a
connected source.
*/
type OperatorKnowledgePassage struct {
	Object    string  `json:"object"`
	Title     string  `json:"title"`
	ArticleID *string `json:"article_id"`
	URL       string  `json:"url"`
	Text      string  `json:"text"`
}

/*
NewOperatorKnowledgePassages shapes passages.
*/
func NewOperatorKnowledgePassages(found []operator.Passage) []OperatorKnowledgePassage {
	out := make([]OperatorKnowledgePassage, len(found))
	for i, p := range found {
		out[i] = OperatorKnowledgePassage{
			Object: "knowledge_passage", Title: p.Title, ArticleID: optionalID(models.PrefixArticle, p.Article),
			URL: p.URL, Text: p.Text,
		}
	}
	return out
}
