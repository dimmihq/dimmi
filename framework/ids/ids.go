/*
|--------------------------------------------------------------------------
| Identifiers
|--------------------------------------------------------------------------
|
| Every row is keyed by a version 7 UUID. Its leading bits are the
| creation time in milliseconds, so new keys land at the end of an index
| instead of scattering across it, and sorting by key sorts by age.
| Imported rows keep their original timestamps, so they sort among the
| rows created natively as if they had always been here.
|
*/

package ids

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

/*
ErrInvalid is returned for text that is not an id at all.
*/
var ErrInvalid = errors.New("ids: invalid id")

/*
UUID is a 128-bit identifier.
*/
type UUID = uuid.UUID

/*
New returns a version 7 UUID for the current time. UUIDs made by one
process always increase, even within the same millisecond. Randomness
comes from crypto/rand, which does not fail on supported platforms.
*/
func New() UUID {
	return uuid.Must(uuid.NewV7())
}

/*
NewV7At returns a version 7 UUID carrying the given time to the
millisecond, with the rest random. Times before 1970 are stored as 1970.
*/
func NewV7At(t time.Time) UUID {
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(max(t.UnixMilli(), 0)))

	id := uuid.Must(uuid.NewRandom())
	copy(id[:6], ms[2:])
	id[6] = 0x70 | id[6]&0x0f

	return id
}

/*
Parse reads a UUID in its standard hyphenated form, or any other form
the uuid package accepts.
*/
func Parse(s string) (UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return Nil(), fmt.Errorf("parse %q: %w", s, ErrInvalid)
	}

	return id, nil
}

/*
Nil returns the all-zero UUID, which never names a row.
*/
func Nil() UUID {
	return uuid.Nil
}
