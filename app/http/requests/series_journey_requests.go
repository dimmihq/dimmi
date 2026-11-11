/*
|--------------------------------------------------------------------------
| Series Journey Requests
|--------------------------------------------------------------------------
|
| The inputs of the people on a series and of one person's journeys.
|
*/

package requests

/*
SeriesEnrollmentsRequest narrows a series' journeys by step and status.
*/
type SeriesEnrollmentsRequest struct {
	Series string `path:"series"`
	Node   string `query:"node"`
	Status string `query:"status" enum:"active,waiting,finished,exited,"`
	Limit  int    `query:"limit"`
	Offset int    `query:"offset"`
}

/*
EnrollSeriesRequest is the input of POST /series/{series}/enrollments:
contacts, or companies for an account series, by public id.
*/
type EnrollSeriesRequest struct {
	Series string `path:"series"`
	Body   struct {
		ContactIDs []string `json:"contact_ids,omitempty" maxItems:"100"`
		CompanyIDs []string `json:"company_ids,omitempty" maxItems:"100"`
	}
}

/*
SeriesEnrollmentRequest names one journey of a series.
*/
type SeriesEnrollmentRequest struct {
	Series     string `path:"series"`
	Enrollment string `path:"enrollment"`
}

/*
ContactSeriesRequest names a contact whose journeys to list.
*/
type ContactSeriesRequest struct {
	Contact string `path:"contact"`
}

/*
CompanySeriesRequest names a company whose journeys to list.
*/
type CompanySeriesRequest struct {
	Company string `path:"company"`
}
