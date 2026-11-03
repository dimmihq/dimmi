/*
|--------------------------------------------------------------------------
| Personal Access Token Resources
|--------------------------------------------------------------------------
|
| A token as its owner and admins see it: its name, the prefix it is
| recognized by, what it may do, when it expires and was last used, and
| the member it acts as. The secret appears once, in the answer that
| minted it, and never in any other answer.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/tokens"
	"github.com/dimmihq/dimmi/app/models"
)

/*
PersonalAccessToken is one token. PlainTextToken is set only in the
answer that minted it.
*/
type PersonalAccessToken struct {
	ID             string      `json:"id"`
	Object         string      `json:"object"`
	Name           string      `json:"name"`
	Prefix         string      `json:"prefix"`
	Abilities      []string    `json:"abilities"`
	ExpiresAt      *time.Time  `json:"expires_at"`
	LastUsedAt     *time.Time  `json:"last_used_at"`
	LastUsedIP     *string     `json:"last_used_ip"`
	RevokedAt      *time.Time  `json:"revoked_at"`
	Member         TokenMember `json:"member"`
	CreatedAt      time.Time   `json:"created_at"`
	PlainTextToken string      `json:"plain_text_token,omitempty"`
}

/*
TokenMember is the member a token acts as, by name.
*/
type TokenMember struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

/*
TokenSummary names the token a request came with, in the me resource.
*/
type TokenSummary struct {
	ID        string     `json:"id"`
	Object    string     `json:"object"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	ExpiresAt *time.Time `json:"expires_at"`
}

/*
TokenAbility is one ability group a token may be given: the permissions
it carries and whether the caller's role allows it.
*/
type TokenAbility struct {
	Object      string   `json:"object"`
	Ability     string   `json:"ability"`
	Permissions []string `json:"permissions"`
	Allowed     bool     `json:"allowed"`
}

/*
NewPersonalAccessToken shapes a token without its secret.
*/
func NewPersonalAccessToken(t tokens.Token) PersonalAccessToken {
	return PersonalAccessToken{
		ID:         models.PublicID(models.PrefixPersonalToken, t.ID),
		Object:     "personal_access_token",
		Name:       t.Name,
		Prefix:     t.Hint,
		Abilities:  nonNil(t.Abilities),
		ExpiresAt:  utc(t.ExpiresAt),
		LastUsedAt: utc(t.LastUsedAt),
		LastUsedIP: t.LastUsedIp,
		RevokedAt:  utc(t.RevokedAt),
		Member: TokenMember{
			ID: models.PublicID(models.PrefixMember, t.OwnerID), Object: "member", Name: t.MemberName,
		},
		CreatedAt: t.CreatedAt.UTC(),
	}
}

/*
NewMintedPersonalAccessToken shapes a token just minted, with the secret
shown this once.
*/
func NewMintedPersonalAccessToken(made tokens.NewToken) PersonalAccessToken {
	out := NewPersonalAccessToken(made.Token)
	out.PlainTextToken = made.Plain

	return out
}

/*
NewPersonalAccessTokens shapes a list of tokens, in the order given.
*/
func NewPersonalAccessTokens(list []tokens.Token) []PersonalAccessToken {
	out := make([]PersonalAccessToken, 0, len(list))
	for _, t := range list {
		out = append(out, NewPersonalAccessToken(t))
	}

	return out
}

/*
NewTokenAbilities shapes the ability groups, in the order given.
*/
func NewTokenAbilities(list []tokens.TokenAbility) []TokenAbility {
	out := make([]TokenAbility, 0, len(list))
	for _, a := range list {
		out = append(out, TokenAbility{
			Object: "token_ability", Ability: a.Ability, Permissions: nonNil(a.Permissions), Allowed: a.Allowed,
		})
	}

	return out
}
