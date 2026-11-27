/*
|--------------------------------------------------------------------------
| Help Center Theme Requests
|--------------------------------------------------------------------------
|
| What the theme endpoints take: a theme's zip as a multipart upload, the
| version to make live, or none for the default theme, and the team's
| values for a theme's settings.
|
*/

package requests

import fhttp "github.com/dimmihq/dimmi/framework/http"

/*
StoreHelpCenterThemeRequest is the input of POST
/help-centers/{help_center}/themes: a multipart form with the theme's
zip in the file field.
*/
type StoreHelpCenterThemeRequest struct {
	HelpCenter string `path:"help_center"`
	RawBody    fhttp.MultipartForm
}

/*
HelpCenterThemeRequest names one theme version of a help center.
*/
type HelpCenterThemeRequest struct {
	HelpCenter string `path:"help_center"`
	Theme      string `path:"theme"`
}

/*
HelpCenterThemeArchiveRequest names a theme version of a help center, or
default for the default theme.
*/
type HelpCenterThemeArchiveRequest struct {
	HelpCenter string `path:"help_center"`
	Theme      string `path:"theme" doc:"A theme version's id, or default for the default theme."`
}

/*
UpdateLiveThemeRequest is the input of PATCH
/help-centers/{help_center}/live-theme: the version the help center's
pages wear, null for the default theme.
*/
type UpdateLiveThemeRequest struct {
	HelpCenter string `path:"help_center"`
	Body       struct {
		ThemeID *string `json:"theme_id" doc:"The theme version to wear, null for the default theme."`
	}
}

/*
UpdateHelpCenterThemeRequest is the input of PATCH
/help-centers/{help_center}/themes/{theme}: the team's values for the
settings the theme declares.
*/
type UpdateHelpCenterThemeRequest struct {
	HelpCenter string `path:"help_center"`
	Theme      string `path:"theme"`
	Body       struct {
		Settings map[string]any `json:"settings" validate:"required" doc:"Values to change, by setting id."`
	}
}
