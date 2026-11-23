/*
|--------------------------------------------------------------------------
| Names
|--------------------------------------------------------------------------
|
| Tools and their arguments are named for what they are, as plainly as
| the words allow, so the weakest model fills them right: a tool that
| reads an article by its id is get_article_by_id and takes article_id.
| Threads written before a name changed still hold the old one, and a
| model that reads them may send it again, so a former tool name still
| finds its tool and a former argument name is read as the new one.
|
*/

package operator

import "encoding/json"

/*
formerNames are the names tools had before, each with its name now.
*/
func formerNames() map[string]string {
	return map[string]string{
		"get_article":            "get_article_by_id",
		"get_procedure":          "get_procedure_by_id",
		"get_conversation":       "get_conversation_by_number",
		"get_customer":           "get_customer_by_conversation_number",
		"agent_runs":             "get_ai_agent_runs",
		"workspace_report":       "get_workspace_report",
		"github_list_merged_prs": "github_list_merged_pull_requests",
		"github_get_pr_diff":     "github_get_pull_request_diff",
		"routine_get_state":      "get_routine_state",
		"routine_set_state":      "save_routine_state",
		"mark_noop":              "report_nothing_to_change",
		"list_recent_changesets": "list_recent_suggestions",
	}
}

/*
lookup finds a tool by its name, or by a name it had before.
*/
func lookup(registry map[string]tool, name string) (tool, bool) {
	if t, ok := registry[name]; ok {
		return t, true
	}
	t, ok := registry[formerNames()[name]]
	return t, ok
}

/*
renamed reads a call's arguments with every former name as its name
now; an argument sent under both keeps the one sent under the new name.
*/
func renamed(args json.RawMessage, was map[string]string) json.RawMessage {
	var in map[string]json.RawMessage
	if len(was) == 0 || json.Unmarshal(args, &in) != nil {
		return args
	}
	changed := false
	for old, now := range was {
		value, sent := in[old]
		if !sent {
			continue
		}
		if _, has := in[now]; !has {
			in[now] = value
		}
		delete(in, old)
		changed = true
	}
	if !changed {
		return args
	}
	out, err := json.Marshal(in)
	if err != nil {
		return args
	}
	return out
}

/*
wasNumber is the former name of a conversation's number.
*/
func wasNumber() map[string]string {
	return map[string]string{"number": "conversation_number"}
}

/*
wasTitle is the former name of a suggestion's title.
*/
func wasTitle() map[string]string {
	return map[string]string{"title": "suggestion_title"}
}
