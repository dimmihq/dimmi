/*
|--------------------------------------------------------------------------
| Domain Names
|--------------------------------------------------------------------------
|
| A sending domain is stored and published in its ASCII form, the only
| one DNS and mail servers read, while people should see the letters
| they typed. A name that cannot be shown otherwise is shown as stored.
|
*/

package mail

import "golang.org/x/net/idna"

/*
UnicodeDomain returns the domain as people write it, so
"xn--caf-dma.example" reads "café.example".
*/
func UnicodeDomain(domain string) string {
	unicode, err := idna.Lookup.ToUnicode(domain)
	if err != nil {
		return domain
	}

	return unicode
}
