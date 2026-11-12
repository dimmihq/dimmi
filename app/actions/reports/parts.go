/*
|--------------------------------------------------------------------------
| Chart Parts
|--------------------------------------------------------------------------
|
| A report whose charts read several independent queries can hand them
| over as parts. The registry then reads every part at the same time,
| each in its own transaction, so the page waits for the slowest chart
| rather than all of them; a source without parts is read in one go.
|
*/

package reports

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
ChartPart reads some of a report's charts.
*/
type ChartPart func(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error)

/*
PartedSource is a source whose charts come in parts that can be read at
once.
*/
type PartedSource interface {
	/*
		ChartParts returns the parts, in the order the charts show.
	*/
	ChartParts() []ChartPart
}

/*
sequential reads the parts one after another in one transaction.
*/
func sequential(ctx context.Context, tx *database.Tx, q Query, parts []ChartPart) ([]Chart, error) {
	var out []Chart

	for _, part := range parts {
		charts, err := part(ctx, tx, q)
		if err != nil {
			return nil, err
		}

		out = append(out, charts...)
	}

	return out, nil
}
