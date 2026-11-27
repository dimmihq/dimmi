/*
|--------------------------------------------------------------------------
| Tool Catalog: Help Center Themes
|--------------------------------------------------------------------------
|
| The entries for a help center's themes: the versions uploaded, the
| team's values for their settings, and the version its pages wear.
| Uploading a theme and downloading one move zip files, which the dimmi
| CLI sends and receives, so they are not tools.
|
*/

package mcpserver

/*
helpCenterThemeDocs returns the entries for help centers' themes.
*/
func helpCenterThemeDocs() map[string]Doc {
	return map[string]Doc{
		"help-centers.themes.index": {
			Title:   "List a help center's themes",
			Use:     "Lists every theme version uploaded to a help center, newest first, with the one readers see.",
			Returns: "the theme versions, live set on the one the help center's pages wear.",
		},
		"help-centers.themes.store": {
			Title: "Upload a help center theme",
			Use: "Uploads a theme's folder as a zip in the file field of a multipart form. dimmi checks every " +
				"file and parses every template; a theme with problems is refused with each one's file and " +
				"line, and nothing is kept. A theme that passes is kept as a new version, not yet live.",
			Returns: "the new theme version.",
		},
		"help-centers.themes.show": {
			Title:   "Get a help center theme",
			Use:     "Reads one theme version of a help center: its name, settings and the team's values.",
			Returns: "the theme version.",
		},
		"help-centers.themes.update": {
			Title: "Change a theme's settings",
			Use: "Sets the team's values for the settings a theme version declares in its theme.json. A value " +
				"that does not fit its setting is refused.",
			Returns: "the theme version with its new values.",
			Params:  map[string]string{"settings": "Values to change, by setting id."},
		},
		"help-centers.themes.archive.show": {
			Title:   "Download a help center theme",
			Use:     "Downloads a theme version's files as a zip, or the default theme's with default as the id.",
			Returns: "the theme's folder as a zip file.",
		},
		"help-centers.theme-preview.show": {
			Title: "Build a help center page for a theme preview",
			Use: "Builds one page of the help center with its live content, as a theme would render it, without " +
				"rendering it. The dimmi CLI renders the page in the theme on the developer's computer.",
			Returns: "the page's status, its redirect when it has one, and the template and variables it renders.",
			Params:  map[string]string{"path": "The page's path, such as /articles/install or /fr."},
		},
		"help-centers.live-theme.update": {
			Title: "Change the theme a help center wears",
			Use: "Makes a theme version the one every page of the help center renders in, or the default " +
				"theme again with a null theme_id. Readers see the change at once.",
			Returns: "the help center with its theme_id.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"theme_id": "The theme version to wear, null for the default theme."},
		},
	}
}
