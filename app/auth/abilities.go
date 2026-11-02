/*
|--------------------------------------------------------------------------
| Abilities
|--------------------------------------------------------------------------
|
| An ability is what a token or an OAuth client may be granted: a group
| of permissions, such as conversations:write. Tokens never name single
| permissions, so a finer permission added later joins an existing group
| and every token minted before keeps meaning the same thing. The
| wildcard grants every group, including groups added after the token
| was minted.
|
*/

package auth

import "slices"

/*
AllAbilities is the wildcard ability that grants every group.
*/
const AllAbilities = "*"

/*
AbilityGroups returns every ability, in the order clients list them.
*/
func AbilityGroups() []string {
	return []string{
		"conversations:read", "conversations:write",
		"contacts:read", "contacts:write",
		"articles:read", "articles:write",
		"members:read", "members:write",
		"settings:read", "settings:write",
		"webhooks:read", "webhooks:write",
		"notifications:read", "notifications:write",
		"ai:read", "ai:write", "ai:keys", "ai:actions",
		"exports:read", "exports:write",
		"changelog:read", "changelog:write",
		"feedback:read", "feedback:write",
		"engage:read", "engage:write",
		"billing:read", "billing:write",
		"system:read", "system:write",
	}
}

/*
ExpandAbilities returns the ability groups the abilities grant: the
wildcard becomes every group, and anything that is not a group is
dropped.
*/
func ExpandAbilities(abilities []string) []string {
	if slices.Contains(abilities, AllAbilities) {
		return AbilityGroups()
	}

	groups := AbilityGroups()

	return slices.DeleteFunc(slices.Clone(abilities), func(a string) bool { return !slices.Contains(groups, a) })
}
