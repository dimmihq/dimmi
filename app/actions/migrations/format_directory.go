/*
|--------------------------------------------------------------------------
| Directory Records
|--------------------------------------------------------------------------
|
| The records the rest name: teammates, teams, tags, custom field
| definitions, companies, contacts and notes about them.
|
*/

package migrations

import "time"

/*
Teammate is a person who worked in the other tool.
*/
type Teammate struct {
	Base
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role,omitempty"`
}

/*
Team is a group of teammates.
*/
type Team struct {
	Base
	Name       string   `json:"name"`
	MemberRefs []string `json:"member_refs,omitempty"`
}

/*
Tag is a label put on contacts, companies and conversations.
*/
type Tag struct {
	Base
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

/*
AttributeDefinition is a custom field of contacts, companies or
conversations.
*/
type AttributeDefinition struct {
	Base
	Object      string   `json:"object"`
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
	Archived    bool     `json:"archived,omitempty"`
}

/*
Company is an organization contacts belong to.
*/
type Company struct {
	Base
	ExternalID       string         `json:"external_id,omitempty"`
	Name             string         `json:"name"`
	Domain           string         `json:"domain,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
	TagRefs          []string       `json:"tag_refs,omitempty"`
	CreatedAt        *time.Time     `json:"created_at,omitempty"`
}
