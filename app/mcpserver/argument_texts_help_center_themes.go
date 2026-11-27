/*
|--------------------------------------------------------------------------
| Written Argument Texts: Help Center Themes
|--------------------------------------------------------------------------
|
| The arguments of the tools that read and change a help center's theme
| versions.
|
*/

package mcpserver

/*
helpCenterThemeTexts returns the texts written for the theme tools.
*/
func helpCenterThemeTexts() map[string]map[string]string {
	theme := "Id of the theme version, starting with thm_. Find it with dimmi_list_help_center_themes."

	return map[string]map[string]string{
		"get_help_center_theme_by_id": {"theme": theme},
		"update_help_center_theme":    {"theme": theme},
	}
}
