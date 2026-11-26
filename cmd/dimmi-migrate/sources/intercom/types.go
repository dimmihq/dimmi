package intercom

import (
	"encoding/json"
	"strconv"
	"strings"
)

/*
ID is an Intercom id, which the API sends as a string in some places
and a number in others.
*/
type ID string

func (id *ID) UnmarshalJSON(raw []byte) error {
	s := strings.TrimSpace(string(raw))

	switch {
	case s == "null":
		*id = ""
	case strings.HasPrefix(s, `"`):
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}

		*id = ID(v)
	default:
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return err
		}

		*id = ID(n.String())
	}

	return nil
}

/*
IDs is a list of ids.
*/
type IDs []ID

func (ids IDs) strings() []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			out = append(out, string(id))
		}
	}

	return out
}

/*
Time is a Unix time the API sends as a number, sometimes a string.
*/
type Time int64

func (t *Time) UnmarshalJSON(raw []byte) error {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		*t = 0

		return nil
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}

	*t = Time(int64(f))

	return nil
}

type me struct {
	ID    ID     `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	App   struct {
		IDCode   string `json:"id_code"`
		Name     string `json:"name"`
		Region   string `json:"region"`
		Timezone string `json:"timezone"`
	} `json:"app"`
}

type admin struct {
	Type    string `json:"type"`
	ID      ID     `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	TeamIDs IDs    `json:"team_ids"`
	Avatar  struct {
		ImageURL string `json:"image_url"`
	} `json:"avatar"`
	Role *struct {
		Name string `json:"name"`
	} `json:"role"`
}

type team struct {
	ID       ID     `json:"id"`
	Name     string `json:"name"`
	AdminIDs IDs    `json:"admin_ids"`
}

type tag struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
}

type dataAttribute struct {
	ID          ID       `json:"id"`
	Model       string   `json:"model"`
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	DataType    string   `json:"data_type"`
	Options     []string `json:"options"`
	Custom      bool     `json:"custom"`
	Archived    bool     `json:"archived"`
	UpdatedAt   Time     `json:"updated_at"`
}

type conversationAttribute struct {
	ID          ID     `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DataType    string `json:"data_type"`
	Required    bool   `json:"required"`
	Archived    bool   `json:"archived"`
	Multiline   bool   `json:"multiline"`
	UpdatedAt   Time   `json:"updated_at"`
	Options     []struct {
		ID       ID     `json:"id"`
		Label    string `json:"label"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		Archived bool   `json:"archived"`
	} `json:"options"`
}

type list[T any] struct {
	Data  []T `json:"data"`
	Pages struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
		Next       json.RawMessage
	} `json:"pages"`
	HasMore    bool   `json:"has_more"`
	TotalCount int    `json:"total_count"`
	Scroll     string `json:"scroll_param"`
}

/*
nextCursor reads pages.next, which is an object holding starting_after
on cursor lists and a URL string on page lists.
*/
func nextCursor(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var obj struct {
		StartingAfter string `json:"starting_after"`
	}

	if json.Unmarshal(raw, &obj) == nil {
		return obj.StartingAfter
	}

	return ""
}

type company struct {
	ID              ID             `json:"id"`
	CompanyID       string         `json:"company_id"`
	Name            string         `json:"name"`
	Website         string         `json:"website"`
	CreatedAt       Time           `json:"created_at"`
	RemoteCreatedAt Time           `json:"remote_created_at"`
	UpdatedAt       Time           `json:"updated_at"`
	LastRequestAt   Time           `json:"last_request_at"`
	Size            *float64       `json:"size"`
	Industry        string         `json:"industry"`
	MonthlySpend    *float64       `json:"monthly_spend"`
	SessionCount    *float64       `json:"session_count"`
	UserCount       *float64       `json:"user_count"`
	CustomAttrs     map[string]any `json:"custom_attributes"`
	Plan            json.RawMessage
	Tags            struct {
		Tags []tag `json:"tags"`
	} `json:"tags"`
}

type ref struct {
	Type string `json:"type"`
	ID   ID     `json:"id"`
}

type refList struct {
	Data    []ref `json:"data"`
	HasMore bool  `json:"has_more"`
	URL     string
}

type contact struct {
	ID               ID              `json:"id"`
	ExternalID       string          `json:"external_id"`
	Role             string          `json:"role"`
	Email            string          `json:"email"`
	Phone            string          `json:"phone"`
	WhatsappNumber   string          `json:"whatsapp_number"`
	Name             string          `json:"name"`
	Avatar           json.RawMessage `json:"avatar"`
	Unsubscribed     bool            `json:"unsubscribed_from_emails"`
	CreatedAt        Time            `json:"created_at"`
	UpdatedAt        Time            `json:"updated_at"`
	SignedUpAt       Time            `json:"signed_up_at"`
	LastSeenAt       Time            `json:"last_seen_at"`
	LanguageOverride string          `json:"language_override"`
	BrowserLanguage  string          `json:"browser_language"`
	CustomAttrs      map[string]any  `json:"custom_attributes"`
	Tags             refList         `json:"tags"`
	Companies        refList         `json:"companies"`
	Location         struct {
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
	} `json:"location"`
}

type note struct {
	ID        ID     `json:"id"`
	CreatedAt Time   `json:"created_at"`
	Body      string `json:"body"`
	Author    struct {
		ID ID `json:"id"`
	} `json:"author"`
	Contact *struct {
		ID ID `json:"id"`
	} `json:"contact"`
}

type attachment struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filesize    int64  `json:"filesize"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type author struct {
	Type        string `json:"type"`
	ID          ID     `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	FromAIAgent bool   `json:"from_ai_agent"`
	IsAIAnswer  bool   `json:"is_ai_answer"`
}

type part struct {
	ID              ID              `json:"id"`
	PartType        string          `json:"part_type"`
	Body            string          `json:"body"`
	CreatedAt       Time            `json:"created_at"`
	UpdatedAt       Time            `json:"updated_at"`
	AssignedTo      *ref            `json:"assigned_to"`
	Author          author          `json:"author"`
	Attachments     []attachment    `json:"attachments"`
	Redacted        bool            `json:"redacted"`
	State           string          `json:"state"`
	Tags            []tag           `json:"tags"`
	EventDetails    json.RawMessage `json:"event_details"`
	EmailMetadata   *emailMeta      `json:"email_message_metadata"`
	PreviousState   string          `json:"previous_ticket_state"`
	TicketState     string          `json:"ticket_state"`
	UpdatedAttrData json.RawMessage `json:"updated_attribute_data"`
}

type emailMeta struct {
	MessageID string `json:"message_id"`
	Subject   string `json:"subject"`
}

type conversation struct {
	ID              ID             `json:"id"`
	Title           string         `json:"title"`
	CreatedAt       Time           `json:"created_at"`
	UpdatedAt       Time           `json:"updated_at"`
	SnoozedUntil    Time           `json:"snoozed_until"`
	State           string         `json:"state"`
	Priority        string         `json:"priority"`
	AdminAssigneeID ID             `json:"admin_assignee_id"`
	TeamAssigneeID  ID             `json:"team_assignee_id"`
	CustomAttrs     map[string]any `json:"custom_attributes"`
	Tags            struct {
		Tags []tag `json:"tags"`
	} `json:"tags"`
	Rating *struct {
		Rating    int    `json:"rating"`
		Remark    string `json:"remark"`
		CreatedAt Time   `json:"created_at"`
		Teammate  *ref   `json:"teammate"`
	} `json:"conversation_rating"`
	Source struct {
		Type          string       `json:"type"`
		ID            ID           `json:"id"`
		DeliveredAs   string       `json:"delivered_as"`
		Subject       string       `json:"subject"`
		Body          string       `json:"body"`
		Author        author       `json:"author"`
		Attachments   []attachment `json:"attachments"`
		URL           string       `json:"url"`
		Redacted      bool         `json:"redacted"`
		EmailMetadata *emailMeta   `json:"email_message_metadata"`
	} `json:"source"`
	Contacts struct {
		Contacts []ref `json:"contacts"`
	} `json:"contacts"`
	Company *struct {
		ID ID `json:"id"`
	} `json:"company"`
	Parts struct {
		Parts      []part `json:"conversation_parts"`
		TotalCount int    `json:"total_count"`
	} `json:"conversation_parts"`
}

type ticketState struct {
	ID            ID     `json:"id"`
	Category      string `json:"category"`
	InternalLabel string `json:"internal_label"`
	ExternalLabel string `json:"external_label"`
}

type ticketTypeAttr struct {
	ID                ID     `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	DataType          string `json:"data_type"`
	Default           bool   `json:"default"`
	VisibleToContacts bool   `json:"visible_to_contacts"`
	RequiredToCreate  bool   `json:"required_to_create"`
	Archived          bool   `json:"archived"`
	Input             struct {
		Multiline bool `json:"multiline"`
		Options   []struct {
			Label    string `json:"label"`
			Archived bool   `json:"archived"`
		} `json:"list_options"`
	} `json:"input_options"`
}

type ticketType struct {
	ID          ID     `json:"id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	UpdatedAt   Time   `json:"updated_at"`
	Attributes  struct {
		Data []ticketTypeAttr `json:"ticket_type_attributes"`
	} `json:"ticket_type_attributes"`
	States struct {
		Data []ticketState `json:"data"`
	} `json:"ticket_states"`
}

type ticket struct {
	ID              ID             `json:"id"`
	TicketID        string         `json:"ticket_id"`
	Category        string         `json:"category"`
	Attributes      map[string]any `json:"ticket_attributes"`
	State           ticketState    `json:"ticket_state"`
	Type            ticketType     `json:"ticket_type"`
	AdminAssigneeID ID             `json:"admin_assignee_id"`
	TeamAssigneeID  ID             `json:"team_assignee_id"`
	CreatedAt       Time           `json:"created_at"`
	UpdatedAt       Time           `json:"updated_at"`
	Open            bool           `json:"open"`
	SnoozedUntil    Time           `json:"snoozed_until"`
	CompanyID       ID             `json:"company_id"`
	Contacts        struct {
		Contacts []ref `json:"contacts"`
	} `json:"contacts"`
	Linked struct {
		Data []ref `json:"data"`
	} `json:"linked_objects"`
	Parts struct {
		Parts      []part `json:"ticket_parts"`
		TotalCount int    `json:"total_count"`
	} `json:"ticket_parts"`
	Tags struct {
		Tags []tag `json:"tags"`
	} `json:"tags"`
}

type helpCenter struct {
	ID           ID       `json:"id"`
	Identifier   string   `json:"identifier"`
	DisplayName  string   `json:"display_name"`
	URL          string   `json:"url"`
	CustomDomain string   `json:"custom_domain"`
	Default      bool     `json:"default"`
	Locales      []string `json:"locales"`
	UpdatedAt    Time     `json:"updated_at"`
}

type groupContent struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type collection struct {
	ID            ID                         `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	CreatedAt     Time                       `json:"created_at"`
	UpdatedAt     Time                       `json:"updated_at"`
	URL           string                     `json:"url"`
	Icon          string                     `json:"icon"`
	Order         int                        `json:"order"`
	DefaultLocale string                     `json:"default_locale"`
	Translated    map[string]json.RawMessage `json:"translated_content"`
	ParentID      ID                         `json:"parent_id"`
	HelpCenterID  ID                         `json:"help_center_id"`
}

type articleContent struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Body         string `json:"body"`
	BodyMarkdown string `json:"body_markdown"`
	AuthorID     ID     `json:"author_id"`
	State        string `json:"state"`
	CreatedAt    Time   `json:"created_at"`
	UpdatedAt    Time   `json:"updated_at"`
	URL          string `json:"url"`
}

type article struct {
	ID            ID                         `json:"id"`
	Title         string                     `json:"title"`
	Description   string                     `json:"description"`
	Body          string                     `json:"body"`
	BodyMarkdown  string                     `json:"body_markdown"`
	AuthorID      ID                         `json:"author_id"`
	State         string                     `json:"state"`
	CreatedAt     Time                       `json:"created_at"`
	UpdatedAt     Time                       `json:"updated_at"`
	URL           string                     `json:"url"`
	ParentID      ID                         `json:"parent_id"`
	ParentIDs     IDs                        `json:"parent_ids"`
	ParentType    string                     `json:"parent_type"`
	DefaultLocale string                     `json:"default_locale"`
	Audience      string                     `json:"help_center_audience"`
	Translated    map[string]json.RawMessage `json:"translated_content"`
	Tags          struct {
		Tags []tag `json:"tags"`
	} `json:"tags"`
}

type internalArticle struct {
	ID        ID     `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	OwnerID   ID     `json:"owner_id"`
	AuthorID  ID     `json:"author_id"`
	CreatedAt Time   `json:"created_at"`
	UpdatedAt Time   `json:"updated_at"`
	Locale    string `json:"locale"`
}

type snippet struct {
	ID           ID     `json:"id"`
	Title        string `json:"title"`
	Locale       string `json:"locale"`
	BodyMarkdown string `json:"body_markdown"`
	CreatedAt    Time   `json:"created_at"`
	UpdatedAt    Time   `json:"updated_at"`
}

type macro struct {
	ID          ID       `json:"id"`
	Name        string   `json:"name"`
	Body        string   `json:"body"`
	BodyText    string   `json:"body_text"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	VisibleTo   string   `json:"visible_to"`
	TeamIDs     []string `json:"visible_to_team_ids"`
	AvailableOn []string `json:"available_on"`
}

type interval struct {
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

type schedule struct {
	ID         ID         `json:"id"`
	Name       string     `json:"name"`
	TimeZone   string     `json:"time_zone_name"`
	Intervals  []interval `json:"time_intervals"`
	AlwaysOpen bool       `json:"twenty_four_seven"`
	UpdatedAt  Time       `json:"updated_at"`
}

type scheduleException struct {
	ID                ID         `json:"id"`
	Date              string     `json:"exception_date"`
	Type              string     `json:"exception_type"`
	Name              string     `json:"name"`
	Intervals         []interval `json:"time_intervals"`
	RecurringAnnually bool       `json:"recurring_annually"`
}
