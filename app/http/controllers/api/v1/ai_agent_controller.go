/*
|--------------------------------------------------------------------------
| AI Agent Controller
|--------------------------------------------------------------------------
|
| The agents API: list and make agents, read and edit one's working
| copy, publish it or turn it off, its versions, the templates a new
| agent starts from, and the workspace's shared AI settings. Every id in
| and out is a public one; the actions decide who may do what.
|
*/

package v1

import (
	"context"
	"math"
	"strconv"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIAgentController serves the agents API.
*/
type AIAgentController struct {
	Agents *ai.Agents
	IDs    *ids.Registry
}

/*
Index lists the workspace's agents, the default first.
*/
func (c *AIAgentController) Index(ctx context.Context, _ *requests.AIAgentsRequest) (*AIAgentsOutput, error) {
	list, err := c.Agents.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	items := make([]resources.AIAgent, len(list))
	for i, s := range list {
		items[i] = resources.NewAIAgentSummary(s)
	}
	page := pagination.Page[resources.AIAgent]{Items: items, Limit: len(items)}
	return &AIAgentsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Templates lists what a new agent can start from.
*/
func (c *AIAgentController) Templates(_ context.Context, _ *requests.AIAgentTemplatesRequest,
) (*AIAgentTemplatesOutput, error) {
	items := resources.NewAIAgentTemplates(ai.Templates())
	page := pagination.Page[resources.AIAgentTemplate]{Items: items, Limit: len(items)}
	return &AIAgentTemplatesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store makes a new agent as a draft from a template.
*/
func (c *AIAgentController) Store(ctx context.Context, in *requests.AIStoreAgentRequest) (*AIAgentOutput, error) {
	view, err := c.Agents.Create(ctx, ai.CreateAgentInput{
		Membership: membership(ctx), Name: in.Body.Name, TemplateKey: in.Body.Template,
	})
	return agentOutput(view, err)
}

/*
Show answers one agent ready to edit.
*/
func (c *AIAgentController) Show(ctx context.Context, in *requests.AIShowAgentRequest) (*AIAgentOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	return agentOutput(c.Agents.Show(ctx, membership(ctx), id))
}

/*
Update saves a change to the agent's working copy.
*/
func (c *AIAgentController) Update(ctx context.Context, in *requests.AIUpdateAgentRequest) (*AIAgentOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	patch, err := c.agentPatch(in.Body, requests.Sent(in.RawBody))
	if err != nil {
		return nil, err
	}
	input := ai.UpdateAgentInput{Membership: membership(ctx), AgentID: id, Patch: patch}
	if in.Body.LockVersion != nil {
		input.LockVersion = *in.Body.LockVersion
	}
	return agentOutput(c.Agents.Update(ctx, input))
}

/*
Destroy deletes an agent that is not the default.
*/
func (c *AIAgentController) Destroy(ctx context.Context, in *requests.AIShowAgentRequest) (*EmptyOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	if err := c.Agents.Delete(ctx, membership(ctx), id); err != nil {
		return nil, err
	}
	return &EmptyOutput{}, nil
}

/*
Publish makes the agent's working copy what customers get.
*/
func (c *AIAgentController) Publish(ctx context.Context, in *requests.AIPublishAgentRequest) (*AIAgentOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	input := ai.PublishInput{Membership: membership(ctx), AgentID: id}
	if in.Body != nil {
		input.ChangeNote = in.Body.ChangeNote
	}
	return agentOutput(c.Agents.Publish(ctx, input))
}

/*
Unpublish turns the agent off.
*/
func (c *AIAgentController) Unpublish(ctx context.Context, in *requests.AIShowAgentRequest) (*AIAgentOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	return agentOutput(c.Agents.Pause(ctx, membership(ctx), id))
}

/*
Versions lists an agent's published versions, newest first.
*/
func (c *AIAgentController) Versions(ctx context.Context, in *requests.AIAgentVersionsRequest,
) (*AIAgentVersionsOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	limit := in.Limit
	if limit == 0 {
		limit = 25
	}
	list, err := c.Agents.Versions(ctx, membership(ctx), id, ai.VersionPage{Before: in.Before, Limit: limit + 1})
	if err != nil {
		return nil, err
	}
	next := ""
	if len(list) > int(limit) {
		list = list[:limit]
		next = strconv.Itoa(int(list[len(list)-1].Version.Version))
	}
	items := make([]resources.AIAgentVersion, len(list))
	for i, v := range list {
		items[i] = resources.NewAIAgentVersion(v)
	}
	page := pagination.Page[resources.AIAgentVersion]{Items: items, Limit: int(limit), NextCursor: next}
	return &AIAgentVersionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
ShowVersion answers one published version.
*/
func (c *AIAgentController) ShowVersion(ctx context.Context, in *requests.AIAgentVersionRequest,
) (*AIAgentVersionOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	v, err := c.Agents.Version(ctx, membership(ctx), id, in.Version)
	if err != nil {
		return nil, err
	}
	return &AIAgentVersionOutput{Body: fhttp.NewResource(resources.NewAIAgentVersion(*v))}, nil
}

/*
RestoreVersion puts a version back as the working copy, or publishes
it.
*/
func (c *AIAgentController) RestoreVersion(ctx context.Context, in *requests.AIRestoreAgentVersionRequest,
) (*AIAgentOutput, error) {
	id, err := c.agentID(in.Agent)
	if err != nil {
		return nil, err
	}
	input := ai.RestoreInput{Membership: membership(ctx), AgentID: id, Version: in.Version}
	if in.Body != nil {
		input.Publish = in.Body.Publish
	}
	return agentOutput(c.Agents.Restore(ctx, input))
}

/*
Settings answers the workspace's shared AI settings.
*/
func (c *AIAgentController) Settings(ctx context.Context, _ *requests.AISettingsRequest) (*AISettingsOutput, error) {
	s, err := c.Agents.Settings(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	return &AISettingsOutput{Body: fhttp.NewResource(resources.NewAISettings(s))}, nil
}

/*
UpdateSettings saves the workspace's shared AI settings.
*/
func (c *AIAgentController) UpdateSettings(ctx context.Context, in *requests.AIUpdateSettingsRequest,
) (*AISettingsOutput, error) {
	sent := requests.Sent(in.RawBody)
	b := in.Body
	p := ai.SettingsPatch{
		AnswerModel:      ai.Optional[string]{Set: sent["answer_model"], Value: b.AnswerModel},
		ClassifierModel:  ai.Optional[string]{Set: sent["classifier_model"], Value: b.ClassifierModel},
		FallbackModel:    ai.Optional[string]{Set: sent["fallback_model"], Value: b.FallbackModel},
		MonthlyCapMicros: capMicros(sent, b.MonthlyCapUSD),
	}
	refs := []struct {
		field string
		value *string
		dst   *ai.Optional[ids.UUID]
	}{
		{"answer_key_id", b.AnswerKeyID, &p.AnswerKeyID},
		{"classifier_key_id", b.ClassifierKeyID, &p.ClassifierKeyID},
		{"fallback_key_id", b.FallbackKeyID, &p.FallbackKeyID},
	}
	for _, ref := range refs {
		decoded, err := c.optionalID(sent, ref.field, models.PrefixAiProviderKey, ref.value)
		if err != nil {
			return nil, err
		}
		*ref.dst = decoded
	}
	s, err := c.Agents.UpdateSettings(ctx, membership(ctx), p)
	if err != nil {
		return nil, err
	}
	return &AISettingsOutput{Body: fhttp.NewResource(resources.NewAISettings(s))}, nil
}

/*
agentOutput shapes an agent view, or passes the error on.
*/
func agentOutput(view *ai.AgentView, err error) (*AIAgentOutput, error) {
	if err != nil {
		return nil, err
	}
	return &AIAgentOutput{Body: fhttp.NewResource(resources.NewAIAgentView(*view))}, nil
}

/*
agentID decodes an agent's public id; anything else is not found.
*/
func (c *AIAgentController) agentID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixAiAgent), public)
	if err != nil {
		return ids.UUID{}, ai.ErrAgentNotFound
	}
	return id, nil
}

/*
agentPatch maps the request to the action's patch: plain fields as
sent, nullable fields set or cleared, public ids decoded.
*/
func (c *AIAgentController) agentPatch(b requests.AIUpdateAgentBody, sent map[string]bool) (ai.AgentPatch, error) {
	p := ai.AgentPatch{
		Name: b.Name, Tone: b.Tone, Guidance: b.Guidance, Description: b.Description, Schedule: b.Schedule,
		AssignedBehavior: b.AssignedBehavior, UnknownBehavior: b.UnknownBehavior,
		DefaultLanguage: b.DefaultLanguage, IncludeContact: b.IncludeContact,
		RequireCitations: b.RequireCitations, PiiRedaction: b.PiiRedaction,
		Streaming: b.Streaming, RaisePrio: b.RaisePriority,
		EnabledChannels: b.EnabledChannels, Languages: b.Languages, LinkAllowlist: b.LinkAllowlist,
		ProductName:   ai.Optional[string]{Set: sent["product_name"], Value: b.ProductName},
		AnswerModel:   ai.Optional[string]{Set: sent["answer_model"], Value: b.AnswerModel},
		FallbackModel: ai.Optional[string]{Set: sent["fallback_model"], Value: b.FallbackModel},
		MaxAiReplies:  b.MaxAiReplies,
		HandoffMode:   handoffMode(b.HandoffMode),
	}

	tags, err := c.idList(sent, "always_handoff_tag_ids", models.PrefixTag, b.AlwaysHandoffTag)
	if err != nil {
		return p, err
	}
	p.AlwaysHandoffTags = tags

	transfers, err := c.idList(sent, "transfer_agent_ids", models.PrefixAiAgent, b.TransferAgentIDs)
	if err != nil {
		return p, err
	}
	p.TransferAgentIDs = transfers

	refs := []struct {
		field  string
		prefix models.Prefix
		value  *string
		dst    *ai.Optional[ids.UUID]
	}{
		{"answer_key_id", models.PrefixAiProviderKey, b.AnswerKeyID, &p.AnswerKeyID},
		{"fallback_key_id", models.PrefixAiProviderKey, b.FallbackKeyID, &p.FallbackKeyID},
		{"handoff_team_id", models.PrefixTeam, b.HandoffTeamID, &p.HandoffTeamID},
	}
	for _, ref := range refs {
		decoded, err := c.optionalID(sent, ref.field, ref.prefix, ref.value)
		if err != nil {
			return p, err
		}
		*ref.dst = decoded
	}

	if err := c.scopes(b, &p); err != nil {
		return p, err
	}
	return p, nil
}

/*
scopes decodes what the agent may use and where it answers.
*/
func (c *AIAgentController) scopes(b requests.AIUpdateAgentBody, p *ai.AgentPatch) error {
	var err error
	if p.Knowledge, err = c.knowledgeScope(b.KnowledgeScope); err != nil {
		return err
	}
	if p.Procedures, err = c.idScope("procedure_scope", models.PrefixProcedure, b.ProcedureScope); err != nil {
		return err
	}
	if p.Actions, err = c.idScope("action_scope", models.PrefixAiAction, b.ActionScope); err != nil {
		return err
	}
	p.Placement, err = c.placement(b.Placement)
	return err
}

/*
knowledgeScope decodes what the agent may search, if the request sent
it.
*/
func (c *AIAgentController) knowledgeScope(k *requests.AIKnowledgeScopeBody) (*agents.KnowledgeScope, error) {
	if k == nil {
		return nil, nil
	}
	centers, err := c.decodeAll("knowledge_scope", models.PrefixHelpCenter, k.HelpCenterIDs)
	if err != nil {
		return nil, err
	}
	sources, err := c.decodeAll("knowledge_scope", models.PrefixKnowledgeSource, k.SourceIDs)
	if err != nil {
		return nil, err
	}
	return &agents.KnowledgeScope{Mode: k.Mode, HelpCenterIDs: centers, SourceIDs: sources}, nil
}

/*
idScope decodes what the agent may use of one kind, if the request sent
it.
*/
func (c *AIAgentController) idScope(field string, prefix models.Prefix, s *requests.AIScopeBody,
) (*agents.Scope, error) {
	if s == nil {
		return nil, nil
	}
	list, err := c.decodeAll(field, prefix, s.IDs)
	if err != nil {
		return nil, err
	}
	return &agents.Scope{Mode: s.Mode, IDs: list}, nil
}

/*
placement decodes where the agent answers, if the request sent it.
*/
func (c *AIAgentController) placement(pl *requests.AIPlacementBody) (*agents.Placement, error) {
	if pl == nil {
		return nil, nil
	}
	conns, err := c.decodeAll("placement", models.PrefixChannelConnection, pl.ChannelConnectionIDs)
	if err != nil {
		return nil, err
	}
	rooms, err := c.decodeAll("placement", models.PrefixChannelRoom, pl.ChannelRoomIDs)
	if err != nil {
		return nil, err
	}
	inboxes, err := c.decodeAll("placement", models.PrefixInbox, pl.InboxIDs)
	if err != nil {
		return nil, err
	}
	return &agents.Placement{ChannelConnectionIDs: conns, ChannelRoomIDs: rooms, InboxIDs: inboxes}, nil
}

/*
decodeAll decodes a list of public ids of one kind.
*/
func (c *AIAgentController) decodeAll(field string, prefix models.Prefix, list []string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(list))
	for _, v := range list {
		id, err := c.IDs.Decode(string(prefix), v)
		if err != nil {
			return nil, fault.New(fault.Invalid, "invalid_id", "the id is not valid").OnField(field)
		}
		out = append(out, id)
	}
	return out, nil
}

/*
idList decodes a list of public ids the request may have sent; a list
sent as null clears it.
*/
func (c *AIAgentController) idList(sent map[string]bool, field string, prefix models.Prefix, list *[]string,
) (*[]ids.UUID, error) {
	if !sent[field] {
		return nil, nil
	}
	if list == nil {
		out := []ids.UUID{}
		return &out, nil
	}
	out, err := c.decodeAll(field, prefix, *list)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

/*
optionalID decodes a nullable public id the request may have sent; null
or an empty string clears it.
*/
func (c *AIAgentController) optionalID(sent map[string]bool, field string, prefix models.Prefix,
	value *string,
) (ai.Optional[ids.UUID], error) {
	if !sent[field] {
		return ai.Optional[ids.UUID]{}, nil
	}
	if value == nil || *value == "" {
		return ai.Optional[ids.UUID]{Set: true}, nil
	}
	id, err := c.IDs.Decode(string(prefix), *value)
	if err != nil {
		return ai.Optional[ids.UUID]{}, fault.New(fault.Invalid, "invalid_id", "the id is not valid").OnField(field)
	}
	return ai.Optional[ids.UUID]{Set: true, Value: &id}, nil
}

/*
handoffMode maps the agent's role the request sent, if any.
*/
func handoffMode(mode *string) *enums.AIHandoffMode {
	if mode == nil {
		return nil
	}
	m := enums.AIHandoffMode(*mode)
	return &m
}

/*
capMicros maps the cap in dollars to micro-dollars; null clears it.
*/
func capMicros(sent map[string]bool, usd *float64) ai.Optional[int64] {
	if !sent["monthly_cap_usd"] || usd == nil {
		return ai.Optional[int64]{Set: sent["monthly_cap_usd"]}
	}
	micros := int64(math.Round(*usd * 1_000_000))
	return ai.Optional[int64]{Set: true, Value: &micros}
}
