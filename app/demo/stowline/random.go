/*
|--------------------------------------------------------------------------
| Seeded Random Source
|--------------------------------------------------------------------------
|
| The seed must write the same workspace for the same seed on every
| machine and every Go release, so it draws from its own small generator,
| SplitMix64, rather than a library whose sequence may change. It is
| for picking demo data only, never for anything secret.
|
*/

package stowline

/*
Random is a seeded source of numbers.
*/
type Random struct {
	state uint64
}

/*
NewRandom returns a source that always draws the same numbers for the
same seed.
*/
func NewRandom(seed uint64) *Random {
	return &Random{state: seed ^ 0x9e3779b97f4a7c15}
}

/*
Uint64 draws the next number.
*/
func (r *Random) Uint64() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

/*
Int64N draws a number from 0 up to n, n excluded; n must be positive.
*/
func (r *Random) Int64N(n int64) int64 {
	return int64(r.Uint64() % uint64(n)) //nolint:gosec // n is positive and the result is below it
}

/*
IntN draws a number from 0 up to n, n excluded; n must be positive.
*/
func (r *Random) IntN(n int) int {
	return int(r.Int64N(int64(n)))
}

/*
Float64 draws a number from 0 up to 1, 1 excluded.
*/
func (r *Random) Float64() float64 {
	return float64(r.Uint64()>>11) / (1 << 53)
}
