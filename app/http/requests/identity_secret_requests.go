/*
|--------------------------------------------------------------------------
| Identity Secret Requests
|--------------------------------------------------------------------------
|
| The inputs of the identity secret endpoints. A shared secret is never
| sent here: the server makes it. A public key arrives as PEM text, and
| fields left out of an update stay as they are.
|
*/

package requests

/*
IdentitySecretRequest names a secret in the path.
*/
type IdentitySecretRequest struct {
	Secret string `path:"secret"`
}

/*
StoreIdentitySecretBody is the body of POST /identity-secrets.
*/
type StoreIdentitySecretBody struct {
	Name      string `json:"name,omitempty"       maxLength:"60"`
	Algorithm string `json:"algorithm"            enum:"HS256,RS256,EdDSA"`
	PublicKey string `json:"public_key,omitempty" maxLength:"16384"`
}

/*
StoreIdentitySecretRequest is the input of POST /identity-secrets.
*/
type StoreIdentitySecretRequest struct {
	Body StoreIdentitySecretBody
}

/*
RotateIdentitySecretBody is the body of POST
/identity-secrets/{secret}/rotations: the new public key, for RS256 and
EdDSA secrets.
*/
type RotateIdentitySecretBody struct {
	PublicKey string `json:"public_key,omitempty" maxLength:"16384"`
}

/*
RotateIdentitySecretRequest is the input of POST
/identity-secrets/{secret}/rotations.
*/
type RotateIdentitySecretRequest struct {
	Secret string `path:"secret"`
	Body   RotateIdentitySecretBody
}
