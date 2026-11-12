/*
|--------------------------------------------------------------------------
| Help Center Site Requests
|--------------------------------------------------------------------------
|
| The inputs of adding and deleting help centers, of the domain a help
| center answers on, of reading its analytics, and of the messenger
| telling which article a visitor opened.
|
*/

package requests

/*
StoreHelpCenterRequest is the input of POST /help-centers. A subdomain
left out is taken from the name.
*/
type StoreHelpCenterRequest struct {
	Body struct {
		Name      string `json:"name"                minLength:"1" maxLength:"80"`
		Subdomain string `json:"subdomain,omitempty" maxLength:"40"`
		Locale    string `json:"locale,omitempty"    maxLength:"16"`
	}
}

/*
DestroyHelpCenterRequest is the input of DELETE
/help-centers/{help_center}: the help center's name typed back.
*/
type DestroyHelpCenterRequest struct {
	HelpCenter string `path:"help_center"`
	Confirm    string `query:"confirm" maxLength:"80"`
}

/*
StoreHelpCenterDomainRequest is the input of PUT
/help-centers/{help_center}/domain.
*/
type StoreHelpCenterDomainRequest struct {
	HelpCenter string `path:"help_center"`
	Body       struct {
		Hostname string `json:"hostname" minLength:"4" maxLength:"253"`
	}
}

/*
HelpCenterAnalyticsRequest is the input of GET
/help-centers/{help_center}/analytics: how many days back, 30 when left
out.
*/
type HelpCenterAnalyticsRequest struct {
	HelpCenter string `path:"help_center"`
	Days       int    `query:"days" minimum:"0" maximum:"90"`
}

/*
StoreWidgetArticleViewRequest is the input of POST
/widget/v1/article-views: the slug of the article the visitor opened.
*/
type StoreWidgetArticleViewRequest struct {
	Body struct {
		Article string `json:"article" minLength:"1" maxLength:"200" pattern:"^[a-z0-9-]+$"`
	}
}
