/*
|--------------------------------------------------------------------------
| Thread Keys
|--------------------------------------------------------------------------
|
| A conversation remembers where its replies go as a thread key. Today
| that is always a person: the customer's id as the Page or Instagram
| account knows them, which only means something on that one account.
| The key names its kind first, so a thread that starts from something
| else, such as a comment on a post, can be told apart later without
| changing the keys already stored.
|
*/

package social

import "strings"

const directPrefix = "dm:"

/*
Target is where a reply goes: Customer, the person's id on the account.
*/
type Target struct {
	Customer string
}

/*
ThreadKey is the key of the customer's direct thread on an account.
*/
func ThreadKey(customer string) string {
	return directPrefix + customer
}

/*
ParseThreadKey reads where a thread's replies go.
*/
func ParseThreadKey(key string) (Target, bool) {
	customer, ok := strings.CutPrefix(key, directPrefix)

	return Target{Customer: customer}, ok && customer != ""
}
