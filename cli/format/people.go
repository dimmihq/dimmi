package format

/*
Author is who wrote an entry or made a change: a teammate or a contact
by source id, an AI agent with the label it showed, or the system.
*/
type Author struct {
	Type  string `json:"type"`
	Ref   string `json:"ref,omitempty"`
	Label string `json:"label,omitempty"`
}

/*
The kinds of author.
*/
const (
	AuthorTeammate = "teammate"
	AuthorContact  = "contact"
	AuthorAIAgent  = "ai_agent"
	AuthorSystem   = "system"
)

/*
Teammate is a person who worked in the source tool. Teammates arrive
disabled; an admin enables them. Current says whether the source still
lists them.
*/
type Teammate struct {
	Base
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role,omitempty"`
	Current   *bool  `json:"current,omitempty"`
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
The objects an attribute describes.
*/
const (
	ObjectContact      = "contact"
	ObjectCompany      = "company"
	ObjectConversation = "conversation"
)

/*
The attribute types.
*/
const (
	TypeText     = "string"
	TypeNumber   = "number"
	TypeDecimal  = "number"
	TypeBoolean  = "boolean"
	TypeDate     = "date"
	TypeList     = "list"
	TypeFiles    = "files"
	TypeRelation = "relationship"
)

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
	Multiline   bool     `json:"multiline,omitempty"`
	Required    bool     `json:"required_to_close,omitempty"`
}
