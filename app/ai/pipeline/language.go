/*
|--------------------------------------------------------------------------
| Language And Default Lines
|--------------------------------------------------------------------------
|
| The answer prompt names the reply language in words ("German"), from
| the classifier's detection when it is a language we know, else the
| agent's default. The default lines a reply falls back to when the
| model wrote none come from the prompt version's defaults file, and
| the workspace can override each one in the agent's messages.
|
*/

package pipeline

import (
	"encoding/json"
	"strings"
)

/*
replyLanguage is the language code the reply should use.
*/
func replyLanguage(e *env, c classification) string {
	code := strings.ToLower(strings.TrimSpace(c.Language))
	if _, ok := languageNames()[code]; ok {
		return code
	}
	return e.Agent.DefaultLanguage
}

/*
languageName spells a language code out; an unknown code stays as is.
*/
func languageName(code string) string {
	primary, _, _ := strings.Cut(strings.ToLower(code), "-")
	if name, ok := languageNames()[primary]; ok {
		return name
	}
	if code == "" {
		return "English"
	}
	return code
}

/*
languageNames maps the primary subtags customers write most to names.
*/
func languageNames() map[string]string {
	return map[string]string{
		"ar": "Arabic", "bg": "Bulgarian", "ca": "Catalan", "cs": "Czech", "da": "Danish",
		"de": "German", "el": "Greek", "en": "English", "es": "Spanish", "et": "Estonian",
		"fa": "Persian", "fi": "Finnish", "fr": "French", "he": "Hebrew", "hi": "Hindi",
		"hr": "Croatian", "hu": "Hungarian", "id": "Indonesian", "it": "Italian", "ja": "Japanese",
		"ko": "Korean", "lt": "Lithuanian", "lv": "Latvian", "ms": "Malay", "nb": "Norwegian",
		"nl": "Dutch", "no": "Norwegian", "pl": "Polish", "pt": "Portuguese", "ro": "Romanian",
		"ru": "Russian", "sk": "Slovak", "sl": "Slovenian", "sr": "Serbian", "sv": "Swedish",
		"th": "Thai", "tr": "Turkish", "uk": "Ukrainian", "vi": "Vietnamese", "zh": "Chinese",
	}
}

/*
line is the customer line for a key: the agent's override, the prompt
version's default, or a last plain fallback.
*/
func (r *Runner) line(e *env, key string) string {
	var overrides map[string]any
	if json.Unmarshal(e.Agent.Messages, &overrides) == nil {
		if v, ok := overrides[key].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	if v := e.Defaults[key]; v != "" {
		return v
	}
	return "A teammate will take it from here."
}
