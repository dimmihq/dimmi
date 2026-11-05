/*
|--------------------------------------------------------------------------
| Contact Exporter
|--------------------------------------------------------------------------
|
| A copy of what a workspace holds about a customer has to include what
| every part of the product kept: their changelog subscriptions, feedback
| votes, survey answers. Each part that stores a customer's data
| registers an exporter, which adds one named section, with any files,
| to the zip. A failing exporter fails the export, which is tried again.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ExportFile is a file an exporter adds under its section's folder.
*/
type ExportFile struct {
	Name     string
	Contents []byte
}

/*
ExportSection is what one part of the product holds about a customer:
Data is written as <name>.json, and Files under <name>/.
*/
type ExportSection struct {
	Data  any
	Files []ExportFile
}

/*
ContactExporter adds what one part of the product holds about a
customer to their export.
*/
type ContactExporter interface {
	/*
		Name is the section's name in the zip, such as "feedback".
	*/
	Name() string

	/*
		Export returns what the part holds about the contact of the
		workspace.
	*/
	Export(ctx context.Context, workspaceID, contactID ids.UUID) (ExportSection, error)
}
