/*
|--------------------------------------------------------------------------
| Terms Resource
|--------------------------------------------------------------------------
|
| An install that publishes legal terms tells the sign-up page where
| they are, so a person can read each document before ticking the box
| that accepts them. An install without legal pages publishes none, and
| its sign-up page asks nothing.
|
*/

package resources

import "github.com/dimmihq/dimmi/config"

/*
Terms is the legal terms a person accepts when creating an account: the
version in force and where to read each document.
*/
type Terms struct {
	Version           string `json:"version"`
	TermsURL          string `json:"terms_url"`
	PrivacyURL        string `json:"privacy_url"`
	DataProcessingURL string `json:"data_processing_url"`
}

/*
NewTerms shapes the legal terms of the install, nil when it publishes
none.
*/
func NewTerms(c config.TermsConfig) *Terms {
	if c.SiteURL == "" {
		return nil
	}

	return &Terms{
		Version:           c.Version,
		TermsURL:          c.SiteURL + "/terms",
		PrivacyURL:        c.SiteURL + "/privacy",
		DataProcessingURL: c.SiteURL + "/privacy#dpa",
	}
}
