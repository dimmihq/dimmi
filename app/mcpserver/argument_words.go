/*
|--------------------------------------------------------------------------
| Words For Arguments Nobody Explained
|--------------------------------------------------------------------------
|
| The API document leaves many request fields without a word. Most say
| enough by their name once it is read the way the API names things: an
| id under another name (assignee_id is a teammate), a list of ids, a
| length of time by its unit, the name or title of the thing a tool is
| about, or a word that means the same wherever it appears. Each gets a
| sentence with what a value looks like, so no argument is left to a
| guess.
|
*/

package mcpserver

import (
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
idAliases are the names an id goes by in a request field beside the
record's own, with the record each one is.
*/
func idAliases() map[string]string {
	return map[string]string{
		"assignee": "member", "owner": "member", "reassign_to_member": "member", "breach_team": "team",
		"access_team": "team", "ticket_type": "type", "default_state": "state", "default_resolved_state": "state",
		"sla_policy": "policy", "custom_role": "role", "source_conversation": "conversation", "impact_field": "field",
		"restore_version": "version", "label": "label", "field": "field", "state": "state", "type": "type",
	}
}

/*
wordGuide explains arguments that mean the same in every tool, by name.
*/
func wordGuide() map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]string{
		"icon":        "An icon as one emoji, such as 📦.",
		"external_id": "Your own system's id of this record, such as user_8421.",
		"slug": "The part of the address that names it, in lowercase words joined by hyphens, such as " +
			"refund-policy.",
		"email":      "An email address, such as ana@example.com.",
		"from_email": "An email address, such as support@example.com.",
		"phone":      "A phone number with its country code, such as +14155550123.",
		"locale":     "A language code, such as en or pt-BR.",
		"language":   "A language code, such as en or pt_BR.",
		"timezone":   "An IANA time zone, such as Europe/Lisbon.",
		"color":      "A color as a hex code, such as #2563eb.",
		"position":   "Where it sits in the list, counting from 0, such as 2.",
		"lock_version": "The lock_version the record had when you read it, such as 3. It stops you " +
			"overwriting a newer save.",
		"domain":          "A domain name, such as example.com.",
		"currency":        "A three-letter currency code, such as USD.",
		"cron_expression": "A five-field cron expression, such as 0 9 * * 1 for Mondays at nine.",
		"base_url":        "The address every request starts with, such as https://api.example.com.",
		"days":            "How many days back to look, such as 30.",
		"from":            "The first day, such as 2026-01-01.",
		"to":              "The last day, included, such as 2026-01-31.",
		"password":        "The password, as text. It is stored encrypted and never shown again.",
		"token":           "The secret token, as text. It is stored encrypted and never shown again.",
		"api_key":         "The API key, as text. It is stored encrypted and never shown again.",
		"access_token":    "The access token, as text. It is stored encrypted and never shown again.",
		"app_secret":      "The app secret, as text. It is stored encrypted and never shown again.",
		"signing_secret":  "The signing secret, as text. It is stored encrypted and never shown again.",
		"username":        "The user name to sign in with, such as api-user.",
		"prompt":          "What to do, in plain words, such as Check the help articles for facts that went stale.",
		"summary":         "One or two sentences that sum it up.",
		"display_name":    "The name people see, such as Acme Support.",
		"dry_run":         "true to try it out without changing anything. false when left out.",
		"refresh":         "true to read it again from the connected tool instead of the saved copy.",
		"enabled":         "true to turn it on, false to turn it off.",
		"pinned":          "true to pin it to the top, false to unpin it.",
		"shared":          "true to share it with the whole team, false to keep it to yourself.",
		"publish":         "true to publish it at once, false to leave it a draft.",
		"trashed":         "true to list what is in the trash instead.",
		"compare":         "true to set each number beside the same number of days just before.",
		"business_hours":  "true to count time only while office hours were open.",
	}
}

/*
units are the endings that say an argument is a length of time or a
size, with the sentence for each.
*/
func units() map[string]string {
	return map[string]string{
		"_seconds": "A length of time in seconds, such as 3600 for one hour.",
		"_minutes": "A length of time in minutes, such as 30.",
		"_days":    "A number of days, such as 7.",
		"_ms":      "A length of time in milliseconds, such as 10000 for ten seconds.",
		"_bytes":   "A size in bytes, such as 65536.",
	}
}

/*
knownText explains an argument the API document says nothing about,
when its name says what it is: an id, a list of ids, a length of time,
a word of the tool's own thing, or a word that means the same
everywhere. Empty when the name does not say.
*/
func knownText(op mcp.Operation, name, api string, schema map[string]any) string {
	if text := idText(api); text != "" {
		return text
	}

	if text, ok := wordGuide()[name]; ok {
		return text
	}

	for ending, text := range units() {
		if strings.HasSuffix(name, ending) {
			return text
		}
	}

	if word := resourceWord(op.ID); word != "" && name == word+"_"+api && name != api {
		return ownText(strings.ReplaceAll(word, "_", " "), api, schema)
	}

	return ""
}

/*
idText explains an argument that holds an id, or a list of ids, of a
record the argument guide knows, under its own name or another.
*/
func idText(api string) string {
	base, many := strings.CutSuffix(api, "_ids")
	if !many {
		var one bool
		if base, one = strings.CutSuffix(api, "_id"); !one {
			return ""
		}
	}

	if record, ok := idAliases()[base]; ok {
		base = record
	}

	guide, ok := idGuides()[base]
	if !ok {
		return ""
	}

	if !many {
		return guide.String()
	}

	text := "Ids of " + guide.what + "s, as a list, such as [\"" + guide.example() + "\"]."
	if guide.finder != "" {
		text += " Find them with " + guide.finder + "."
	}

	return text
}

/*
ownText explains a word of the thing a tool is about: its name, title,
description, body or subject.
*/
func ownText(thing, word string, schema map[string]any) string {
	text := "The " + word + " of the " + thing
	if word == "body" {
		text = "The text of the " + thing
	}

	if limit, ok := schema["maxLength"].(float64); ok {
		return fmt.Sprintf("%s, at most %d characters.", text, int(limit))
	}

	return text + "."
}

/*
kindOf is the type an argument takes, read past a null it may also be.
*/
func kindOf(schema map[string]any) string {
	if kind, ok := schema["type"].(string); ok {
		return kind
	}

	for _, kind := range asList(schema["type"]) {
		if text := fmt.Sprint(kind); text != "null" {
			return text
		}
	}

	return ""
}
