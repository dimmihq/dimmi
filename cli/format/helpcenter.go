package format

import "time"

/*
HelpCenter is a help center with its default language and the others it
is translated into.
*/
type HelpCenter struct {
	Base
	Name          string   `json:"name"`
	DefaultLocale string   `json:"default_locale,omitempty"`
	Locales       []string `json:"locales,omitempty"`
	URL           string   `json:"url,omitempty"`
}

/*
Translation is a collection's or an article's text in another language.
*/
type Translation struct {
	Locale      string     `json:"locale"`
	Name        string     `json:"name,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Body        string     `json:"body,omitempty"`
	State       string     `json:"state,omitempty"`
	Author      *Author    `json:"author,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	Redirects   []string   `json:"redirects,omitempty"`
}

/*
Collection is a group of articles, nested under another or at the top.
*/
type Collection struct {
	Base
	HelpCenterRef string        `json:"help_center_ref"`
	ParentRef     string        `json:"parent_ref,omitempty"`
	Name          string        `json:"name"`
	Description   string        `json:"description,omitempty"`
	Icon          string        `json:"icon,omitempty"`
	Order         int           `json:"order,omitempty"`
	Translations  []Translation `json:"translations,omitempty"`
}

/*
The article visibilities.
*/
const (
	VisibilityPublic    = "public"
	VisibilityAgentOnly = "agent_only"
)

/*
Article is a help center article. Visibility is public or agent_only;
Redirects are the old paths that should lead to it. Blocks lists the
kinds of rich block its body uses.
*/
type Article struct {
	Base
	HelpCenterRef string        `json:"help_center_ref"`
	CollectionRef string        `json:"collection_ref,omitempty"`
	Title         string        `json:"title"`
	Description   string        `json:"description,omitempty"`
	Body          string        `json:"body"`
	State         string        `json:"state,omitempty"`
	Visibility    string        `json:"visibility,omitempty"`
	Audience      string        `json:"audience,omitempty"`
	Author        *Author       `json:"author,omitempty"`
	CreatedAt     *time.Time    `json:"created_at,omitempty"`
	UpdatedAt     *time.Time    `json:"updated_at,omitempty"`
	Redirects     []string      `json:"redirects,omitempty"`
	Translations  []Translation `json:"translations,omitempty"`
	Blocks        []string      `json:"blocks,omitempty"`
	TagRefs       []string      `json:"tag_refs,omitempty"`
}

/*
Snippet is a piece of text teammates and the AI agent reuse; it becomes
an article only teammates see.
*/
type Snippet struct {
	Base
	HelpCenterRef string     `json:"help_center_ref,omitempty"`
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	Locale        string     `json:"locale,omitempty"`
	Author        *Author    `json:"author,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
}
