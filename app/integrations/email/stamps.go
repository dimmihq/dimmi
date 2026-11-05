/*
|--------------------------------------------------------------------------
| Stamps
|--------------------------------------------------------------------------
|
| Mail the install sends carries stamps only it can make. A loop stamp
| on every reply lets the install recognize its own mail when a
| customer's mailbox forwards it straight back, so it is never taken for
| the customer's answer. A test stamp on a forwarding test proves that
| mail sent to the public address arrived through the forward; each test
| gets its own token, so a returning mail finds its own test. Each is
| an HMAC under a key derived from the application key, bound to what it
| stamps, so it cannot be copied onto other mail or made up.
|
*/

package email

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
Stamp header names and the stamp format version.
*/
const (
	LoopHeader = "X-Dimmi-Loop"
	TestHeader = "X-Dimmi-Test"

	stampVersion = "v1."
	stampLength  = 22

	testTokenVersion = "t1."
	testNonceLength  = 16
	testLinePrefix   = "dimmi test "
)

/*
testTokenPattern matches the shape of a forwarding test token.
*/
var testTokenPattern = regexp.MustCompile(`t1\.[0-9A-Za-z]{16}\.[0-9A-Za-z_-]{22}`)

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
TestToken returns a fresh token of a forwarding test of the address.
*/
func (s Stamps) TestToken(addressID string) string {
	nonce := str.Random(testNonceLength)

	return testTokenVersion + nonce + "." + s.mac("test", addressID, nonce)
}

/*
ValidTestToken reports whether the token is one this install made for a
forwarding test of the address.
*/
func (s Stamps) ValidTestToken(token, addressID string) bool {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 || parts[0]+"." != testTokenVersion || !testTokenPattern.MatchString(token) {
		return false
	}

	return hashing.Equal([]byte(parts[2]), []byte(s.mac("test", addressID, parts[1])))
}

/*
TestLine returns the line that ends a forwarding test's body, so the
token survives a forward that drops extra headers.
*/
func TestLine(token string) string {
	return testLinePrefix + token
}

/*
FindTestToken returns the test token a received mail carries: the
header's, else the subject's for a test sent by hand, else the one on
the last line of the text body.
*/
func FindTestToken(header, subject, text string) string {
	for _, found := range []string{header, subject} {
		if token := testTokenPattern.FindString(found); token != "" {
			return token
		}
	}

	lines := strings.Split(strings.TrimRight(text, " \t\r\n"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])

	if !strings.HasPrefix(strings.ToLower(last), testLinePrefix) {
		return ""
	}

	return testTokenPattern.FindString(last)
}

/*
mac returns the shortened HMAC of the parts, joined so that no two
lists of parts give the same input.
*/
func (s Stamps) mac(parts ...string) string {
	sum := hashing.HMAC(s.key, []byte(strings.Join(parts, "\x00")))

	return base64.RawURLEncoding.EncodeToString(sum)[:stampLength]
}
