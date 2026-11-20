/*
|--------------------------------------------------------------------------
| Changelog Analytics Requests
|--------------------------------------------------------------------------
|
| The input of a changelog's analytics.
|
*/

package requests

/*
ChangelogAnalyticsRequest is the input of GET
/help-centers/{help_center}/changelog-analytics: how many days back, 30
when left out.
*/
type ChangelogAnalyticsRequest struct {
	HelpCenter string `path:"help_center"`
	Days       int    `query:"days" minimum:"0" maximum:"90"`
}
