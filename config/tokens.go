/*
|--------------------------------------------------------------------------
| Personal Access Token Settings
|--------------------------------------------------------------------------
|
| How many working tokens one member may hold. Tokens are cheap to make
| and easy to forget, so a cap keeps a member's list readable and stops
| a runaway script from minting them without end. Revoked and expired
| tokens never count.
|
*/

package config

/*
TokensConfig caps the personal access tokens of a member. MaxPerMember
is how many live tokens one member may hold in a workspace.
*/
type TokensConfig struct {
	MaxPerMember int `default:"50" koanf:"max_per_member"`
}
