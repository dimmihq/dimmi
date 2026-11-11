/*
|--------------------------------------------------------------------------
| Series Outputs
|--------------------------------------------------------------------------
|
| What the series API answers: one series, a page of them, what one
| misses before it can go live, and its versions.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
SeriesListOutput is a page of series.
*/
type SeriesListOutput struct {
	Body fhttp.Collection[resources.Series]
}

/*
SeriesOutput is one series.
*/
type SeriesOutput struct {
	Body fhttp.Resource[resources.Series]
}

/*
SeriesProblemsOutput is what a series misses before it can go live.
*/
type SeriesProblemsOutput struct {
	Body fhttp.Resource[SeriesProblems]
}

/*
SeriesProblems lists a series' problems.
*/
type SeriesProblems struct {
	Object   string           `json:"object"`
	Problems []series.Problem `json:"problems"`
}

/*
SeriesVersionsOutput lists a series' versions.
*/
type SeriesVersionsOutput struct {
	Body fhttp.Collection[resources.SeriesVersion]
}
