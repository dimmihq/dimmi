/*
|--------------------------------------------------------------------------
| Widget Help Requests
|--------------------------------------------------------------------------
|
| The input of the messenger's Help tab, which reads the help center in
| the visitor's language.
|
*/

package requests

/*
ShowWidgetHelpRequest is the input of the messenger's Help tab: the
language to read it in, else the visitor's browser languages.
*/
type ShowWidgetHelpRequest struct {
	Locale         string `query:"locale"          maxLength:"16"`
	AcceptLanguage string `header:"Accept-Language" maxLength:"512"`
}
