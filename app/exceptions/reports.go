/*
|--------------------------------------------------------------------------
| Report Errors
|--------------------------------------------------------------------------
|
| How reports refuse: a range that cannot be read or runs longer than a
| report covers, a metric a report cannot list conversations for, and a
| digest schedule that is not one of the offered ones.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
ErrReportRange refuses a day that is not a date, an end before the start
or a range longer than reports cover; callers name the field.
ErrReportMetric refuses listing the conversations behind a metric the
report does not have. ErrReportDigestFrequency refuses a digest that is
neither daily nor weekly.
*/
var (
	ErrReportRange = fault.New(fault.Invalid, "report_range",
		"Pick a range of up to 400 days, with the end on or after the start.")
	ErrReportMetric = fault.New(fault.Invalid, "report_metric",
		"This report has no such number to list conversations for.").OnField("metric")
	ErrReportDigestFrequency = fault.New(fault.Invalid, "report_digest_frequency",
		"A report digest comes daily or weekly.").OnField("frequency")
)
