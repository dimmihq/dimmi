/*
|--------------------------------------------------------------------------
| Contact Records
|--------------------------------------------------------------------------
|
| Contacts, the companies they belong to, where they were seen, and the
| notes teammates kept about them.
|
*/

package migrations

import "time"

/*
CompanyLink is one company a contact belongs to; one link is the
contact's default.
*/
type CompanyLink struct {
	Ref     string `json:"ref"`
	Default bool   `json:"default,omitempty"`
}

/*
Contact is a customer: a signed-up user or an anonymous lead.
*/
type Contact struct {
	Base
	Kind             string         `json:"kind,omitempty"`
	ExternalID       string         `json:"external_id,omitempty"`
	Email            string         `json:"email,omitempty"`
	Name             string         `json:"name,omitempty"`
	Phone            string         `json:"phone,omitempty"`
	AvatarURL        string         `json:"avatar_url,omitempty"`
	Locale           string         `json:"locale,omitempty"`
	Timezone         string         `json:"timezone,omitempty"`
	Location         *Location      `json:"location,omitempty"`
	FirstSeenAt      *time.Time     `json:"first_seen_at,omitempty"`
	LastSeenAt       *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt        *time.Time     `json:"created_at,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
	TagRefs          []string       `json:"tag_refs,omitempty"`
	Companies        []CompanyLink  `json:"companies,omitempty"`
}

/*
Location is where a contact was last seen.
*/
type Location struct {
	City    string `json:"city,omitempty"`
	Region  string `json:"region,omitempty"`
	Country string `json:"country,omitempty"`
}

/*
ContactNote is a teammate's note about a contact or a company.
*/
type ContactNote struct {
	Base
	ContactRef string    `json:"contact_ref,omitempty"`
	CompanyRef string    `json:"company_ref,omitempty"`
	Author     Author    `json:"author"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}
