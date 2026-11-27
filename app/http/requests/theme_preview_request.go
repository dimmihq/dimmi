/*
|--------------------------------------------------------------------------
| Theme Preview Request
|--------------------------------------------------------------------------
|
| What the theme preview endpoint takes: the path of the page the dimmi
| CLI renders in a theme on the developer's computer.
|
*/

package requests

/*
ThemePreviewRequest is the input of GET
/help-centers/{help_center}/theme-preview: the page to build.
*/
type ThemePreviewRequest struct {
	HelpCenter string `path:"help_center"`
	Path       string `query:"path" maxLength:"2048" doc:"The page's path, such as /articles/install or /fr."`
}
