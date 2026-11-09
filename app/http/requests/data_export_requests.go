/*
|--------------------------------------------------------------------------
| Data Export Requests
|--------------------------------------------------------------------------
|
| Inputs of the data export endpoints. An export names its kind, its
| format and what it covers: the ids picked, a list's filters and search
| as the list endpoint reads them, or a saved view.
|
*/

package requests

/*
StoreDataExportRequest is the input of POST /data-exports. Selection
holds ids, or filter and q, or view_id.
*/
type StoreDataExportRequest struct {
	Body struct {
		Kind         string          `json:"kind" validate:"required,oneof=conversations contacts"`
		Format       string          `json:"format" validate:"required,oneof=txt pdf csv json"`
		IncludeNotes bool            `json:"include_notes,omitempty"`
		Selection    ExportSelection `json:"selection,omitzero"`
	}
}

/*
ExportSelection is what an export covers. Filter takes the same keys and
values as the list's filter[...] parameters, and Conditions the contact
list's conditions parameter.
*/
type ExportSelection struct {
	IDs        []string          `json:"ids,omitempty"        validate:"max=10000"`
	Filter     map[string]string `json:"filter,omitempty"`
	Q          string            `json:"q,omitempty"`
	ViewID     string            `json:"view_id,omitempty"`
	Conditions string            `json:"conditions,omitempty" validate:"max=8000"`
}

/*
DataExportRequest is the input of the endpoints on one export.
*/
type DataExportRequest struct {
	Export string `path:"data_export"`
}
