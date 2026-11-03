//go:build cloud

/*
|--------------------------------------------------------------------------
| Cloud Edition
|--------------------------------------------------------------------------
|
| Built with the cloud tag, the binary is the one Dimmi runs for its
| customers: the self-hosted product plus the cloud-only code, such as
| billing and the marketing site, which live in files carrying the same
| tag.
|
*/

package config

/*
BuiltEdition is the edition this binary was built as.
*/
const BuiltEdition = EditionCloud
