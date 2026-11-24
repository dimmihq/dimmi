/*
|--------------------------------------------------------------------------
| Operator Suggestion Requests
|--------------------------------------------------------------------------
|
| A suggestion an agent doing Operator's work makes, one request per
| kind with everything that kind needs. The MCP catalog says what
| belongs in each field, since a model fills these from the
| description alone.
|
*/

package requests

/*
PassageEditInput replaces one passage of an article.
*/
type PassageEditInput struct {
	Find    string `json:"find"    validate:"required"`
	Replace string `json:"replace"`
}

/*
StoreArticleEditSuggestionRequest is the input of POST
/operator/suggestions/article-edits.
*/
type StoreArticleEditSuggestionRequest struct {
	Body struct {
		ArticleID       string             `json:"article_id"                  validate:"required,max=64"`
		Edits           []PassageEditInput `json:"edits"                       validate:"required,min=1,max=20,dive"`
		SuggestionTitle string             `json:"suggestion_title"            validate:"required,max=200"`
		Reason          string             `json:"reason"                      validate:"required,max=2000"`
		DedupKey        string             `json:"dedup_key,omitempty"         validate:"omitempty,max=200"`
		NewArticleTitle string             `json:"new_article_title,omitempty" validate:"omitempty,max=150"`
		RoutineID       string             `json:"routine_id,omitempty"        validate:"omitempty,max=64"`
		DryRun          bool               `json:"dry_run,omitempty"`
	}
}

/*
StoreNewArticleSuggestionRequest is the input of POST
/operator/suggestions/new-articles.
*/
type StoreNewArticleSuggestionRequest struct {
	Body struct {
		ArticleTitle    string `json:"article_title"            validate:"required,max=150"`
		ArticleBody     string `json:"article_body"             validate:"required,max=262144"`
		SuggestionTitle string `json:"suggestion_title"         validate:"required,max=200"`
		Reason          string `json:"reason"                   validate:"required,max=2000"`
		Visibility      string `json:"visibility,omitempty"     validate:"omitempty,oneof=public agent_only"`
		DedupKey        string `json:"dedup_key,omitempty"      validate:"omitempty,max=200"`
		CollectionID    string `json:"collection_id,omitempty"  validate:"omitempty,max=64"`
		HelpCenterID    string `json:"help_center_id,omitempty" validate:"omitempty,max=64"`
		RoutineID       string `json:"routine_id,omitempty"     validate:"omitempty,max=64"`
		DryRun          bool   `json:"dry_run,omitempty"`
	}
}

/*
StoreProcedureEditSuggestionRequest is the input of POST
/operator/suggestions/procedure-edits.
*/
type StoreProcedureEditSuggestionRequest struct {
	Body struct {
		ProcedureID         string         `json:"procedure_id"         validate:"required,max=64"`
		ProcedureDefinition map[string]any `json:"procedure_definition" validate:"required"`
		SuggestionTitle     string         `json:"suggestion_title"     validate:"required,max=200"`
		Reason              string         `json:"reason"               validate:"required,max=2000"`
		DedupKey            string         `json:"dedup_key,omitempty"  validate:"omitempty,max=200"`
		RoutineID           string         `json:"routine_id,omitempty" validate:"omitempty,max=64"`
		DryRun              bool           `json:"dry_run,omitempty"`
	}
}

/*
StoreChangesSuggestionRequest is the input of POST
/operator/suggestions/changes: changes to conversations and customers,
and regression tests for procedures.
*/
type StoreChangesSuggestionRequest struct {
	Body struct {
		Operations      []map[string]any `json:"operations"           validate:"required,min=1,max=50"`
		SuggestionTitle string           `json:"suggestion_title"     validate:"required,max=200"`
		Reason          string           `json:"reason"               validate:"required,max=2000"`
		DedupKey        string           `json:"dedup_key,omitempty"  validate:"omitempty,max=200"`
		RoutineID       string           `json:"routine_id,omitempty" validate:"omitempty,max=64"`
		DryRun          bool             `json:"dry_run,omitempty"`
	}
}
