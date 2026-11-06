/*
|--------------------------------------------------------------------------
| Data Export Events
|--------------------------------------------------------------------------
|
| A member's export of conversations or customers is asked for, becomes
| ready, and is deleted. The audit log keeps each step, since an export
| takes data out of the workspace. Only the member who asked hears that
| the file is ready.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
DataExportRequested is dispatched when a member asks for an export of
Kind in Format covering Matched rows.
*/
type DataExportRequested struct {
	Base

	ExportID ids.UUID
	Kind     string
	Format   string
	Matched  int
}

/*
EventName names the event.
*/
func (DataExportRequested) EventName() string { return "data_export.requested" }

/*
DataExportReady is dispatched when an export can be downloaded, with
the rows it holds.
*/
type DataExportReady struct {
	Base

	ExportID    ids.UUID
	RequestedBy ids.UUID
	Rows        int
}

/*
EventName names the event.
*/
func (DataExportReady) EventName() string { return "data_export.ready" }

/*
Member returns the member who asked for the export.
*/
func (e DataExportReady) Member() ids.UUID { return e.RequestedBy }

/*
DataExportDeleted is dispatched when a member deletes an export and its
file.
*/
type DataExportDeleted struct {
	Base

	ExportID ids.UUID
}

/*
EventName names the event.
*/
func (DataExportDeleted) EventName() string { return "data_export.deleted" }
