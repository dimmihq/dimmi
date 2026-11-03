/*
|--------------------------------------------------------------------------
| Stamps
|--------------------------------------------------------------------------
|
| Mail the install sends carries stamps only it can make. A loop stamp
| on every reply lets the install recognize its own mail when a
| customer's mailbox forwards it straight back, so it is never taken for
| the customer's answer. A test stamp on a forwarding test proves that
| mail sent to the public address arrived through the forward. Each is
| an HMAC under a key derived from the application key, bound to what it
| stamps, so it cannot be copied onto other mail.
|
*/

package email

import (
	"encoding/base64"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
Stamp header names and the stamp format version.
*/
const (
	LoopHeader = "X-Dimmi-Loop"
	TestHeader = "X-Dimmi-Test"

	stampVersion = "v1."
	stampLength  = 22
)

/*
Stamps makes and checks stamps with one key.
*/
type Stamps struct {
	key []byte
}

/*
NewStamps returns stamps made with the key.
*/
func NewStamps(key []byte) Stamps {
	return Stamps{key: key}
}

/*
Loop returns the loop stamp of a message the workspace sends with the
Message-ID.
*/
func (s Stamps) Loop(workspaceID, messageID string) string {
	return stampVersion + s.mac("loop", workspaceID, messageID)
}

/*
ValidLoop reports whether the stamp is the workspace's own loop stamp
for the Message-ID.
*/
func (s Stamps) ValidLoop(stamp, workspaceID, messageID string) bool {
	return messageID != "" && hashing.Equal([]byte(strings.TrimSpace(stamp)), []byte(s.Loop(workspaceID, messageID)))
}

/*
Test returns the stamp of a forwarding test of the address.
*/
func (s Stamps) Test(addressID string) string {
	return stampVersion + s.mac("test", addressID)
}

/*
ValidTest reports whether the stamp is a forwarding test of the
address.
*/
func (s Stamps) ValidTest(stamp, addressID string) bool {
	return hashing.Equal([]byte(strings.TrimSpace(stamp)), []byte(s.Test(addressID)))
}

/*
mac returns the shortened HMAC of the parts, joined so that no two
lists of parts give the same input.
*/
func (s Stamps) mac(parts ...string) string {
	sum := hashing.HMAC(s.key, []byte(strings.Join(parts, "\x00")))

	return base64.RawURLEncoding.EncodeToString(sum)[:stampLength]
}
