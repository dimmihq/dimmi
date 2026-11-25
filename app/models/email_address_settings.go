/*
|--------------------------------------------------------------------------
| Email Address Settings
|--------------------------------------------------------------------------
|
| The settings an email address keeps as JSON, read with their defaults
| so an address stored before a setting existed behaves as a new one:
| replies quote the previous message, subjects carry no ticket tag, and
| bulk mail that starts no thread opens a closed conversation,
| replies keep the plain look, and nothing answers mail that arrives
| outside office hours.
|
*/

package models

import "encoding/json"

/*
EmailAddressSettings are an address's own settings. QuotePrevious puts
the message answered below each reply. SubjectTag adds the conversation
number to reply subjects. BulkMailHandling is close, open or drop.
Template is the look of replies, plain or branded; a branded reply
uses BrandColor and closes with FooterMarkdown. AwayReply answers a
new conversation that arrives outside office hours with AwayReplyText,
whose {{reply_time}} says when the team is back.
*/
type EmailAddressSettings struct {
	QuotePrevious    bool   `json:"quote_previous"`
	SubjectTag       bool   `json:"subject_tag"`
	BulkMailHandling string `json:"bulk_mail_handling"`
	Template         string `json:"template"`
	BrandColor       string `json:"brand_color"`
	FooterMarkdown   string `json:"footer_markdown"`
	AwayReply        bool   `json:"away_reply"`
	AwayReplyText    string `json:"away_reply_text"`
}

/*
DefaultAwayReplyText is what an away reply says until the address
words its own.
*/
const DefaultAwayReplyText = "Thanks for writing in. We're away right now and will reply {{reply_time}}."

/*
DefaultEmailAddressSettings returns the settings of a new address.
*/
func DefaultEmailAddressSettings() EmailAddressSettings {
	return EmailAddressSettings{
		QuotePrevious: true, BulkMailHandling: "close", Template: "plain", AwayReplyText: DefaultAwayReplyText,
	}
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

	if settings.AwayReplyText == "" {
		settings.AwayReplyText = DefaultAwayReplyText
	}

	return settings
}
