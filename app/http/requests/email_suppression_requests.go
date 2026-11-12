/*
|--------------------------------------------------------------------------
| Email Suppression Requests
|--------------------------------------------------------------------------
|
| Inputs of the email suppression endpoints.
|
*/

package requests

/*
ListEmailSuppressionsRequest names the contact whose address is read.
*/
type ListEmailSuppressionsRequest struct {
	Contact string `query:"filter[contact]" required:"true"`
}

/*
EmailSuppressionRequest names one suppression.
*/
type EmailSuppressionRequest struct {
	Suppression string `path:"email_suppression"`
}
