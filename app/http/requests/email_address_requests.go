/*
|--------------------------------------------------------------------------
| Email Address Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a workspace's email addresses. An address
| is named by its public id. A new address needs only the public
| address customers write to; how replies are signed and which inbox it
| feeds have defaults. An update changes only the fields it carries.
|
*/

package requests

/*
EmailAddressRequest is the input of an endpoint on one address.
*/
type EmailAddressRequest struct {
	Address string `path:"address"`
}

/*
EmailAddressTestRequest names a forwarding test of an address.
*/
type EmailAddressTestRequest struct {
	Address string `path:"address"`
	Test    string `path:"test"`
}

/*
StoreEmailAddressRequest is the input of POST /email-addresses.
*/
type StoreEmailAddressRequest struct {
	Body struct {
		Address     string  `json:"address"                validate:"required,email,max=254"`
		DisplayName *string `json:"display_name,omitempty" validate:"omitempty,max=80"`
		InboxID     *string `json:"inbox_id,omitempty"`
	}
}

/*
UpdateEmailAddressRequest is the input of PATCH /email-addresses/{address}.
*/
type UpdateEmailAddressRequest struct {
	Address string `path:"address"`
	Body    struct {
		DisplayName   *string `json:"display_name,omitempty"    validate:"omitempty,min=1,max=80"`
		ReplyToMode   *string `json:"reply_to_mode,omitempty"   validate:"omitempty,oneof=token none"`
		FromNameStyle *string `json:"from_name_style,omitempty" validate:"omitempty,oneof=member_and_name name_only"`
		IsDefault     *bool   `json:"is_default,omitempty"`
		InboxID       *string `json:"inbox_id,omitempty"`
		Settings      *struct {
			QuotePrevious    *bool   `json:"quote_previous,omitempty"`
			BulkMailHandling *string `json:"bulk_mail_handling,omitempty" validate:"omitempty,oneof=close open drop"`
			ReplyStyle
		} `json:"settings,omitempty"`
	}
}

/*
ListInboundEmailsRequest is the input of
GET /email-addresses/{address}/inbound-emails.
*/
type ListInboundEmailsRequest struct {
	Address string `path:"address"`
}
