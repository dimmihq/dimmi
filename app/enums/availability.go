/*
|--------------------------------------------------------------------------
| Availability
|--------------------------------------------------------------------------
|
| A member says whether they take conversations. Online is only what
| they chose: they count as present only while connected as well.
|
*/

package enums

/*
Availability is what a member chose to show their teammates and
customers.
*/
type Availability string

/*
The availabilities.
*/
const (
	AvailabilityOnline Availability = "online"
	AvailabilityAway   Availability = "away"
)

/*
Availabilities returns every availability, in the order of the database
check.
*/
func Availabilities() []Availability {
	return []Availability{AvailabilityOnline, AvailabilityAway}
}

/*
Valid reports whether the availability is one of the availabilities.
*/
func (a Availability) Valid() bool {
	switch a {
	case AvailabilityOnline, AvailabilityAway:
		return true
	}

	return false
}
