/*
|--------------------------------------------------------------------------
| Written Schema Texts: Help Center Themes
|--------------------------------------------------------------------------
|
| What each field of a help center's theme versions means.
|
*/

package mcpserver

/*
schemaTextsFromHelpCenterTheme returns the texts of help center themes.
*/
func schemaTextsFromHelpCenterTheme() map[string]map[string]string {
	return map[string]map[string]string{
		"HelpCenter": {
			"theme_id": "Id of the theme version the help center's pages wear, starting with thm_. Null " +
				"for the default theme.",
		},
		"HelpCenterTheme": {
			"api_version": "The theme API the theme is written for, from its theme.json.",
			"content_hash": "A hash of every file of the theme; two versions with the same hash hold " +
				"the same files.",
			"file_count":     "How many files the theme holds.",
			"help_center_id": "Id of the help center the theme belongs to, starting with hc_.",
			"live":           "Whether this is the version the help center's pages wear now.",
			"name":           "The theme's name, from its theme.json.",
			"settings": "The settings the theme declares in its theme.json, which the team may change " +
				"without code.",
			"size_bytes":    "The size of all the theme's files together, in bytes.",
			"theme_version": "The theme author's own version label, such as 1.2.0. Empty when not set.",
			"values": "The team's values for the theme's settings, by setting id. Settings left out " +
				"use their defaults.",
			"version": "The upload's number within the help center, counting up from 1.",
		},
		"ThemePreview": {
			"location": "Where the page sends the reader instead, for a redirect. Left out otherwise.",
			"status":   "The HTTP status the page answers with, such as 200, 302 or 404.",
			"template": "The theme template the page renders, such as article. Left out for a redirect.",
			"variables": "The variables the template reads, by name. HTML and times are one-key objects, " +
				"{\"$html\": ...} and {\"$time\": ...}.",
		},
		"HelpCenterThemeSetting": {
			"default": "The value used until the team sets one. Left out when the theme gives none.",
			"info":    "More words for the team about the setting. Left out when the theme gives none.",
			"label":   "The setting's name as the team reads it.",
			"options": "The values a select setting allows. Left out for other types.",
			"type":    "What the setting holds: text, textarea, color, checkbox, number, url or select.",
		},
	}
}
