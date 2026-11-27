/*
|--------------------------------------------------------------------------
| Help Center Forms In Tests
|--------------------------------------------------------------------------
|
| Every form a help center's page posts carries an anti-forgery token
| bound to the reader's browser. A test posts the way a reader's page
| does: it opens a page of the help center with the same cookies, reads
| the token the page carries and sends it along.
|
*/

package tests

import (
	"html"
	"regexp"
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"
)

var formToken = regexp.MustCompile(`<meta name="form-token" content="([^"]+)"`)

/*
HelpForms returns the client carrying the token the help center at base,
such as /hc/acme, gives it, for posting its forms.
*/
func HelpForms(tb testing.TB, client *testkit.Client, base string) *testkit.Client {
	tb.Helper()
	page := client.WithHeader("Accept", "text/html").Send("GET", base+"/search", nil)
	m := formToken.FindStringSubmatch(string(page.Body))
	if m == nil {
		tb.Fatalf("%s/search carries no form token (status %d)", base, page.Status)
	}
	return client.WithHeader("X-Form-Token", html.UnescapeString(m[1]))
}
