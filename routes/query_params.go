/*
|--------------------------------------------------------------------------
| Hand Read Query Parameters
|--------------------------------------------------------------------------
|
| Some handlers read query parameters beside the list ones, such as a
| numbered page or a request for a CSV file. The published API document
| lists them, so clients and agents built from it can send them. The
| parameters several routes share are described once here.
|
*/

package routes

import "github.com/dimmihq/dimmi/framework/routing"

/*
numberedPageQuery documents the numbered page a directory list starts
at.
*/
func numberedPageQuery() routing.QueryParam {
	return routing.QueryParam{Name: "page", Description: "Numbered page to answer, from 1, instead of a cursor."}
}

/*
numberedPagesQuery documents the numbered page a directory list starts
at and the request to count every matching row.
*/
func numberedPagesQuery() []routing.QueryParam {
	return []routing.QueryParam{
		numberedPageQuery(),
		{Name: "total", Description: "Set to true to count the matching rows, up to a cap.", Values: []string{"true"}},
	}
}

/*
csvQuery documents the request to download a list as a CSV file
instead of a page of JSON.
*/
func csvQuery() routing.QueryParam {
	return routing.QueryParam{
		Name: "format", Description: "Set to csv to download every matching row as CSV.", Values: []string{"csv"},
	}
}
