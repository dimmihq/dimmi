/*
|--------------------------------------------------------------------------
| Email Signature Requests
|--------------------------------------------------------------------------
|
| The inputs of the signature routes. Signatures, teammates and addresses
| are named by their public ids.
|
*/

package requests

/*
EmailSignatureRequest names one signature in the path.
*/
type EmailSignatureRequest struct {
	Signature string `path:"signature"`
}

/*
StoreEmailSignatureRequest is the input of POST /email-signatures. Kind
defaults to personal; MemberID names another teammate's.
*/
type StoreEmailSignatureRequest struct {
	Body struct {
		Kind           string  `json:"kind,omitempty"             validate:"omitempty,oneof=personal address"`
		MemberID       *string `json:"member_id,omitempty"        validate:"omitempty,max=64"`
		EmailAddressID *string `json:"email_address_id,omitempty" validate:"omitempty,max=64"`
		Body           string  `json:"body"                       validate:"max=2000"`
		Enabled        *bool   `json:"enabled,omitempty"`
	}
}

/*
UpdateEmailSignatureRequest is the input of PATCH
/email-signatures/{signature}.
*/
type UpdateEmailSignatureRequest struct {
	Signature string `path:"signature"`
	Body      struct {
		Body    *string `json:"body,omitempty"    validate:"omitempty,max=2000"`
		Enabled *bool   `json:"enabled,omitempty"`
	}
}

/*
StoreEmailSignatureRenderingRequest is the input of
POST /email-signatures/renderings.
*/
type StoreEmailSignatureRenderingRequest struct {
	Body struct {
		Body           string  `json:"body"                       validate:"max=2000"`
		EmailAddressID *string `json:"email_address_id,omitempty" validate:"omitempty,max=64"`
	}
}
