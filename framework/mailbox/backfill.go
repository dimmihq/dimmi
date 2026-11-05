/*
|--------------------------------------------------------------------------
| Backfilling Sources
|--------------------------------------------------------------------------
|
| Some providers keep the mail they received, so mail missed while the
| application was down can be asked for again. Handing a mail over twice
| is safe: the application recognizes a mail it already holds by its
| provider id.
|
*/

package mailbox

import (
	"context"
	"time"
)

/*
Received is one mail a provider still holds: its id, when the provider
received it, and its envelope.
*/
type Received struct {
	ID        string
	CreatedAt time.Time
	Envelope  *Envelope
}

/*
Backfiller is a source that can hand over again the mail its provider
holds.
*/
type Backfiller interface {
	/*
		Backfill walks the mail received since the given time and hands
		each one to the function. The function's error stops the walk.
	*/
	Backfill(ctx context.Context, since time.Time, handle func(context.Context, Received) error) error
}
