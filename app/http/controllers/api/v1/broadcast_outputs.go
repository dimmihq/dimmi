/*
|--------------------------------------------------------------------------
| Broadcast Outputs
|--------------------------------------------------------------------------
|
| What the broadcast endpoints answer with: one broadcast, a page of
| them, a report, an estimate of reach and a page of recipients, each
| wrapped the way every API response is.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
BroadcastsOutput is a page of broadcasts.
*/
type BroadcastsOutput struct {
	Body fhttp.Collection[resources.Broadcast]
}

/*
BroadcastOutput is one broadcast.
*/
type BroadcastOutput struct {
	Body fhttp.Resource[resources.Broadcast]
}

/*
BroadcastReportOutput is a broadcast's report.
*/
type BroadcastReportOutput struct {
	Body fhttp.Resource[resources.BroadcastReport]
}

/*
BroadcastEstimateOutput is a broadcast's reach now.
*/
type BroadcastEstimateOutput struct {
	Body fhttp.Resource[resources.BroadcastEstimate]
}

/*
BroadcastRecipientsOutput is a page of a broadcast's recipients.
*/
type BroadcastRecipientsOutput struct {
	Body fhttp.Collection[resources.BroadcastRecipient]
}
