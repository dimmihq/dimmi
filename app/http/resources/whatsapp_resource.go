/*
|--------------------------------------------------------------------------
| WhatsApp Resources
|--------------------------------------------------------------------------
|
| A WhatsApp connection as the settings page shows it: its number, the
| name customers see, its quality and limit, the inbox it feeds, and, for
| a workspace's own Meta app, the address and token its webhook is set
| up with. Its token never appears. A template shows what it needs
| filled in and how the customer reads it; a conversation shows whether
| free text can still go out, and until when.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
WhatsAppChannel is a WhatsApp connection.
*/
type WhatsAppChannel struct {
	ID                 string          `json:"id"`
	Object             string          `json:"object"`
	Name               string          `json:"name"`
	Status             string          `json:"status"`
	Mode               string          `json:"mode"`
	InboxID            string          `json:"inbox_id"`
	DisplayPhoneNumber string          `json:"display_phone_number"`
	VerifiedName       string          `json:"verified_name"`
	NameStatus         string          `json:"name_status"`
	QualityRating      string          `json:"quality_rating"`
	MessagingLimit     string          `json:"messaging_limit"`
	WabaID             string          `json:"waba_id"`
	PhoneNumberID      string          `json:"phone_number_id"`
	Coexistence        bool            `json:"coexistence"`
	Webhook            *WhatsAppHookup `json:"webhook"`
	LastError          *string         `json:"last_error"`
	CreatedAt          time.Time       `json:"created_at"`
}

/*
WhatsAppHookup is where a workspace's own Meta app delivers and the
token Meta echoes when it checks the address.
*/
type WhatsAppHookup struct {
	CallbackURL string `json:"callback_url"`
	VerifyToken string `json:"verify_token"`
}

/*
WhatsAppFacts are what a connection knows of its number, as the
resource shows them.
*/
type WhatsAppFacts struct {
	Mode, DisplayPhoneNumber, VerifiedName, NameStatus, QualityRating, MessagingLimit string
	WabaID, PhoneNumberID                                                             string
	Coexistence                                                                       bool
}

/*
NewWhatsAppChannel returns the resource of the connection.
*/
func NewWhatsAppChannel(conn models.ChannelConnection, f WhatsAppFacts, hook *WhatsAppHookup) WhatsAppChannel {
	return WhatsAppChannel{
		ID: models.PublicID(models.PrefixChannelConnection, conn.ID), Object: "whatsapp_channel", Name: conn.Name,
		Status: conn.Status, Mode: f.Mode, InboxID: models.PublicID(models.PrefixInbox, conn.InboxID),
		DisplayPhoneNumber: f.DisplayPhoneNumber, VerifiedName: f.VerifiedName, NameStatus: f.NameStatus,
		QualityRating: f.QualityRating, MessagingLimit: f.MessagingLimit, WabaID: f.WabaID,
		PhoneNumberID: f.PhoneNumberID, Coexistence: f.Coexistence, Webhook: hook, LastError: conn.LastError,
		CreatedAt: conn.CreatedAt,
	}
}

/*
WhatsAppAccount is a WhatsApp account a token can see, with its numbers.
*/
type WhatsAppAccount struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Business string           `json:"business"`
	Numbers  []WhatsAppNumber `json:"numbers"`
}

/*
WhatsAppNumber is a number on an account and whether it is connected
here already.
*/
type WhatsAppNumber struct {
	ID                 string `json:"id"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
	QualityRating      string `json:"quality_rating"`
	Connected          bool   `json:"connected"`
}
