/*
|--------------------------------------------------------------------------
| Names
|--------------------------------------------------------------------------
|
| A tool is named for what it does, verb first, in the words a person
| would use: dimmi_get_article_by_id, dimmi_list_help_center_articles,
| dimmi_publish_article. The name is worked out from the API operation,
| so a new operation gets a clear name without anyone writing one, and
| the catalog can still give a tool a better name by hand. A model that
| reads only the name should know what the tool does.
|
*/

package mcpserver

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
toolPrefix starts every tool's name, so tools from dimmi stand apart
from an agent's other servers.
*/
const toolPrefix = "dimmi_"

/*
verbs renames the API's action names to the verbs agents expect.
*/
func verbs() map[string]string {
	return map[string]string{
		"index": "list", "show": "get", "store": "create", "update": "update", "destroy": "delete",
		"update_many": "update_many", "destroy_many": "delete_many",
	}
}

/*
actions are the API's way of saying a verb as a record made or removed:
making a publication publishes, removing one unpublishes. Each is keyed
by the last resource and the verb, and names the verb a person would
use on the resource before it.
*/
func actions() map[string]string {
	return map[string]string{
		"publications create": "publish", "publications delete": "unpublish", "applications create": "apply",
		"declines create": "decline", "reversions create": "revert", "restorations create": "restore",
		"restore create": "restore", "pause create": "pause", "resume create": "resume", "send create": "send",
		"resumptions create": "resume",
		"stop create":        "stop", "cancel create": "cancel", "duplicate create": "duplicate",
		"archive create": "archive", "unarchive create": "unarchive", "check create": "check",
		"checks create": "check", "simulate create": "simulate", "run create": "run", "skip create": "skip",
		"syncs create": "sync", "merges create": "merge", "rollbacks create": "roll_back",
		"refresh create": "refresh", "escalations create": "escalate", "test create": "test",
		"tests create": "test", "previews create": "preview", "estimate get": "estimate",
		"rotations create": "rotate",
	}
}

/*
unchanged are resource words that read the same for one and for many.
*/
func unchanged() []string {
	return []string{
		"series", "settings", "analytics", "hours", "access", "stats", "metrics", "preferences", "entitlements",
		"credentials", "usage", "whatsapp", "sla", "onboarding", "status", "address", "knowledge", "feedback",
		"search", "context", "slack", "instagram", "facebook", "ai", "me", "user", "workspace", "neighbors",
	}
}

/*
irregular are resource words whose singular is not the word less an s.
*/
func irregular() map[string]string {
	return map[string]string{
		"companies": "company", "replies": "reply", "entries": "entry", "memories": "memory",
		"policies": "policy", "deliveries": "delivery", "identities": "identity", "summaries": "summary",
		"addresses": "address", "statuses": "status", "inboxes": "inbox", "redeliveries": "redelivery",
		"changesets": "suggestion",
	}
}

/*
plain is a resource word as a tool's name says it: Operator's
changesets are suggestions to everyone who reads them.
*/
func plain(word string) string {
	return strings.ReplaceAll(word, "changesets", "suggestions")
}

/*
singular is a resource word for one of it: help_centers is help_center.
*/
func singular(word string) string {
	head, last := "", word
	if at := strings.LastIndex(word, "_"); at >= 0 {
		head, last = word[:at+1], word[at+1:]
	}

	if slices.Contains(unchanged(), last) {
		return word
	}

	if one, ok := irregular()[last]; ok {
		return head + one
	}

	if strings.HasSuffix(last, "s") && !strings.HasSuffix(last, "ss") {
		return head + strings.TrimSuffix(last, "s")
	}

	return word
}

/*
phrase is an operation as a verb and the resource words after it.
*/
func phrase(operationID string) (string, []string) {
	parts := strings.Split(strings.ReplaceAll(operationID, "-", "_"), ".")
	verb, resource := parts[len(parts)-1], parts[:len(parts)-1]

	if v, ok := verbs()[verb]; ok {
		verb = v
	}

	many := verb == "list" || strings.HasSuffix(verb, "_many")

	if len(resource) > 1 {
		if act, ok := actions()[resource[len(resource)-1]+" "+verb]; ok {
			verb, resource, many = act, resource[:len(resource)-1], false
		}
	}

	words := make([]string, len(resource))
	for i, word := range resource {
		words[i] = singular(word)
		if many && i == len(resource)-1 {
			words[i] = plain(word)
		}
	}

	return verb, words
}

/*
toolName names the tool of an operation: dimmi_get_article_by_id for
articles.show, dimmi_create_conversation_message for
conversations.messages.store, or the name its catalog entry gives it.
*/
func toolName(op mcp.Operation, doc Doc) string {
	if doc.Name != "" {
		return toolPrefix + doc.Name
	}

	verb, words := phrase(op.ID)
	name := toolPrefix + strings.Join(append([]string{verb}, words...), "_")

	if verb == "get" && strings.HasSuffix(op.Path, "}") {
		name += "_by_id"
	}

	return name
}

/*
resourceWord is the one thing an operation is about, as one word: reply
for conversations.replies.store. An argument too bare to read alone,
such as body, is named after it: reply_body.
*/
func resourceWord(operationID string) string {
	_, words := phrase(operationID)
	if len(words) == 0 {
		return ""
	}

	return singular(words[len(words)-1])
}
