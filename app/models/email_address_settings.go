/*
|--------------------------------------------------------------------------
| Email Address Settings
|--------------------------------------------------------------------------
|
| The settings an email address keeps as JSON, read with their defaults
| so an address stored before a setting existed behaves as a new one:
| replies quote the previous message, subjects carry no ticket tag, and
| bulk mail that starts no thread opens a closed conversation.
|
*/

package models

import "encoding/json"

/*
EmailAddressSettings are an address's own settings. QuotePrevious puts
the message answered below each reply. SubjectTag adds the conversation
number to reply subjects. BulkMailHandling is close, open or drop.
*/
type EmailAddressSettings struct {
	QuotePrevious    bool   `json:"quote_previous"`
	SubjectTag       bool   `json:"subject_tag"`
	BulkMailHandling string `json:"bulk_mail_handling"`
}

/*
DefaultEmailAddressSettings returns the settings of a new address.
*/
func DefaultEmailAddressSettings() EmailAddressSettings {
	return EmailAddressSettings{QuotePrevious: true, BulkMailHandling: "close"}
}

/*
ReadEmailAddressSettings returns stored settings with the defaults
filled in for what they lack. Settings that do not read as JSON are the
defaults.
*/
func ReadEmailAddressSettings(raw json.RawMessage) EmailAddressSettings {
	settings := DefaultEmailAddressSettings()
	if len(raw) == 0 {
		return settings
	}

	if err := json.Unmarshal(raw, &settings); err != nil {
		return DefaultEmailAddressSettings()
	}

	if settings.BulkMailHandling == "" {
		settings.BulkMailHandling = "close"
	}

	return settings
}
