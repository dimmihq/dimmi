/*
|--------------------------------------------------------------------------
| AI Requests
|--------------------------------------------------------------------------
|
| The agent's request shapes. A settings save changes only the fields it
| sends; a nullable field sent as null or an empty string is cleared.
| Every value the database would refuse is refused here first, with the
| field it belongs to: tones, schedules and behaviors from their fixed
| sets, channels the product knows, real language tags, and host names
| for the links the agent may share.
|
*/

package requests

import (
	"context"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/validation"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
AIAgentRequest names the agent implicitly: the workspace has one.
*/
type AIAgentRequest struct{}

/*
AIUpdateAgentBody is the body of PATCH /ai/agent. Key and team ids are
public ids; monthly_cap_usd of 0 means no cap.
*/
type AIUpdateAgentBody struct {
	Name             *string   `json:"name,omitempty"                      validate:"omitempty,min=1,max=60"`
	Enabled          *bool     `json:"enabled,omitempty"`
	Tone             *string   `json:"tone,omitempty"                      validate:"omitempty,max=20"`
	Guidance         *string   `json:"guidance,omitempty"                  validate:"omitempty,max=2000"`
	ProductName      *string   `json:"product_name,omitempty"              validate:"omitempty,max=80"`
	EnabledChannels  *[]string `json:"enabled_channels,omitempty"          validate:"omitempty,max=10"`
	Schedule         *string   `json:"schedule,omitempty"`
	AnswerKeyID      *string   `json:"answer_key_id,omitempty"             validate:"omitempty,max=64"`
	AnswerModel      *string   `json:"answer_model,omitempty"              validate:"omitempty,max=120"`
	FallbackKeyID    *string   `json:"fallback_key_id,omitempty"           validate:"omitempty,max=64"`
	FallbackModel    *string   `json:"fallback_model,omitempty"            validate:"omitempty,max=120"`
	MonthlyCapUSD    *float64  `json:"monthly_cap_usd,omitempty"           validate:"omitempty,gte=0,lte=100000"`
	MaxAiReplies     *int32    `json:"max_ai_replies,omitempty"            validate:"omitempty,gte=1,lte=50"`
	RequireCitations *bool     `json:"require_citations,omitempty"`
	PiiRedaction     *bool     `json:"pii_redaction,omitempty"`
	Languages        *[]string `json:"languages,omitempty"                 validate:"omitempty,max=50"`
	DefaultLanguage  *string   `json:"default_language,omitempty"          validate:"omitempty,max=35"`
	UnknownBehavior  *string   `json:"unknown_behavior,omitempty"`
	AssignedBehavior *string   `json:"assigned_behavior,omitempty"`
	Streaming        *bool     `json:"streaming,omitempty"`
	IncludeContact   *string   `json:"include_contact_context,omitempty"`
	LinkAllowlist    *[]string `json:"link_allowlist,omitempty"            validate:"omitempty,max=100"`
	HandoffTeamID    *string   `json:"handoff_team_id,omitempty"           validate:"omitempty,max=64"`
	RaisePriority    *bool     `json:"raise_priority_on_handoff,omitempty"`
}

/*
AIUpdateAgentRequest is the input of PATCH /ai/agent. RawBody tells a
field sent as null from one left out.
*/
type AIUpdateAgentRequest struct {
	Body    AIUpdateAgentBody
	RawBody []byte
}

/*
Validate refuses the values the agent does not understand, field by
field.
*/
func (r *AIUpdateAgentRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	b := r.Body
	enumsByField := []struct {
		field   string
		value   *string
		allowed []string
	}{
		{"tone", b.Tone, AITones()},
		{"schedule", b.Schedule, AISchedules()},
		{"assigned_behavior", b.AssignedBehavior, AIAssignedBehaviors()},
		{"unknown_behavior", b.UnknownBehavior, AIUnknownBehaviors()},
		{"include_contact_context", b.IncludeContact, AIContactContexts()},
	}
	for _, e := range enumsByField {
		if e.value != nil && !slices.Contains(e.allowed, *e.value) {
			return violation(ctx, e.field, "oneof", "validation.oneof")
		}
	}

	if b.EnabledChannels != nil && !allIn(*b.EnabledChannels, AIChannels()) {
		return violation(ctx, "enabled_channels", "oneof", "validation.oneof")
	}
	if b.LinkAllowlist != nil && !allHosts(*b.LinkAllowlist) {
		return violation(ctx, "link_allowlist", "host", "validation.registered")
	}
	if b.Languages != nil && !allLanguages(*b.Languages) {
		return violation(ctx, "languages", "locale", "validation.locale")
	}
	if b.DefaultLanguage != nil && !allLanguages([]string{*b.DefaultLanguage}) {
		return violation(ctx, "default_language", "locale", "validation.locale")
	}
	return nil
}

/*
AITones lists how the agent may sound.
*/
func AITones() []string {
	out := []string{}
	for _, tone := range enums.AITones() {
		out = append(out, string(tone))
	}
	return out
}

/*
AISchedules lists when the agent may answer.
*/
func AISchedules() []string {
	return []string{
		string(enums.AIScheduleAlways), string(enums.AIScheduleOutsideOffice), string(enums.AIScheduleInsideOffice),
	}
}

/*
AIAssignedBehaviors lists what the agent does in a conversation a
teammate holds.
*/
func AIAssignedBehaviors() []string {
	return []string{"skip_when_human_active", "never_when_assigned", "always"}
}

/*
AIUnknownBehaviors lists what the agent does when it cannot answer.
*/
func AIUnknownBehaviors() []string {
	return []string{"offer_handoff", "handoff", "say_unknown"}
}

/*
AIContactContexts lists how much of the contact the agent reads.
*/
func AIContactContexts() []string {
	return []string{"none", "basic"}
}

/*
AIChannels lists the channels the agent can answer on.
*/
func AIChannels() []string {
	return []string{string(enums.ChannelTypeWidget), string(enums.ChannelTypeEmail), string(enums.ChannelTypeAPI)}
}

/*
allIn reports whether every value is allowed.
*/
func allIn(values, allowed []string) bool {
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			return false
		}
	}
	return true
}

/*
allHosts reports whether every value is a host name, optionally a
wildcard over a domain's subdomains such as *.example.com.
*/
func allHosts(values []string) bool {
	host := regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`)
	for _, v := range values {
		if len(v) > 253 || !host.MatchString(v) {
			return false
		}
	}
	return true
}

/*
allLanguages reports whether every value is a language tag.
*/
func allLanguages(values []string) bool {
	for _, v := range values {
		if _, ok := i18n.Canonical(v); !ok || v == "" {
			return false
		}
	}
	return true
}
