/*
|--------------------------------------------------------------------------
| User Behavior
|--------------------------------------------------------------------------
|
| Questions every part of the product asks about an account, answered in
| one place so they never drift: whether it can sign in with a password,
| whether its email is proven, and whether a second factor protects it.
|
*/

package models

/*
HasPassword reports whether the user can sign in with a password. Users
who signed up through a provider or a magic link have none until they
set one.
*/
func (u User) HasPassword() bool {
	return u.PasswordHash != nil && *u.PasswordHash != ""
}

/*
IsVerified reports whether the user proved they receive mail at their
address.
*/
func (u User) IsVerified() bool {
	return u.EmailVerifiedAt != nil
}

/*
TwoFactorEnabled reports whether a second factor protects the account.
*/
func (u User) TwoFactorEnabled() bool {
	return u.TwoFactorConfirmedAt != nil
}

/*
IsDeleted reports whether the account was deleted and its row
anonymized.
*/
func (u User) IsDeleted() bool {
	return u.DeletedAt != nil
}
