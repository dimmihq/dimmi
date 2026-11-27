//go:build dev

/*
|--------------------------------------------------------------------------
| Marketing Demo: Agent And Workflows
|--------------------------------------------------------------------------
|
| The parts of the Lumen workspace the AI agent, Procedures and Workflows
| pages are shot in: two teams to route to, a few tags, the agent's voice
| and rules, one versioned procedure with actions and simulations, and one
| workflow that sorts a first message by topic. Everything is saved
| through the real actions. The agent is left switched off, so nothing
| here calls a model.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
marketingGuidance is what Lumen's support lead wrote for the agent.
*/
const marketingGuidance = `You answer customers of Lumen, an analytics product for software teams.

- Search the help center before answering anything about Lumen, and link the article you used.
- Customers are engineers and product managers. Lead with the fix, then explain.
- Never promise dates, quote prices or discuss legal terms. Hand those to the team.
- If an Enterprise customer reports an outage, hand over at once.`

/*
agentSetup seeds what the agent, procedures and workflow shots need.
*/
func (c lumenSeeder) agentSetup(ctx context.Context, owner authz.Membership) error {
	a := c.App

	teamIDs := map[string]ids.UUID{}
	for _, name := range []string{"Billing", "Engineering"} {
		made, err := a.Actions.CreateTeam.Handle(ctx, teams.CreateTeamInput{Membership: owner, Name: name})
		if err != nil {
			return fmt.Errorf("create team %s: %w", name, err)
		}

		teamIDs[name] = made.Team.ID
	}

	tagIDs := map[string]ids.UUID{}
	for _, t := range [][2]string{{"billing", "blue"}, {"bug", "red"}, {"enterprise", "orange"}} {
		made, err := a.Actions.Inbox.CreateTag.Handle(ctx, tags.CreateTagInput{
			Membership: owner, Name: t[0], Color: new(t[1]),
		})
		if err != nil {
			return fmt.Errorf("create tag %s: %w", t[0], err)
		}

		tagIDs[t[0]] = made.ID
	}

	if err := c.configureAgent(ctx, owner, teamIDs["Engineering"]); err != nil {
		return err
	}

	actions, err := c.actions(ctx, owner)
	if err != nil {
		return err
	}

	if err := c.procedure(ctx, owner, actions, teamIDs); err != nil {
		return err
	}

	return c.workflow(ctx, owner, teamIDs, tagIDs)
}

/*
configureAgent names the workspace's agent, gives it its guidance and
the team it hands over to, and publishes it when it is ready to answer.
*/
func (c lumenSeeder) configureAgent(ctx context.Context, owner authz.Membership, handsTo ids.UUID) error {
	a := c.App

	view, err := a.Actions.AI.Agents.Default(ctx, owner)
	if err != nil {
		return fmt.Errorf("provision agent: %w", err)
	}

	patch := ai.AgentPatch{
		Name: new("Lumi"), Tone: new("friendly"), Guidance: new(marketingGuidance), Schedule: new("always"),
		AssignedBehavior: new("skip_when_human_active"), UnknownBehavior: new("offer_handoff"),
		DefaultLanguage: new("en"), RequireCitations: new(true), EnabledChannels: &[]string{"widget", "email"},
		ProductName:   ai.Optional[string]{Set: true, Value: new("Lumen")},
		HandoffTeamID: ai.Optional[ids.UUID]{Set: true, Value: new(handsTo)},
	}
	update := ai.UpdateAgentInput{Membership: owner, AgentID: view.Agent.ID, Patch: patch}
	if _, err := a.Actions.AI.Agents.Update(ctx, update); err != nil {
		return fmt.Errorf("configure agent: %w", err)
	}
	if !view.Ready {
		return nil
	}
	if _, err := a.Actions.AI.Agents.Publish(ctx, ai.PublishInput{
		Membership: owner, AgentID: view.Agent.ID, ChangeNote: "Set up for the marketing demo",
	}); err != nil {
		return fmt.Errorf("publish agent: %w", err)
	}

	return nil
}

/*
workflow saves and publishes the workflow that sorts a first message by
topic: billing goes to the Billing team, bugs to Engineering at high
priority, and anything else carries on to the inbox.
*/
func (c lumenSeeder) workflow(
	ctx context.Context, owner authz.Membership, teamIDs, tagIDs map[string]ids.UUID,
) error {
	a := c.App
	if a.Actions.Workflows == nil {
		return nil
	}

	raw, _ := json.Marshal(marketingWorkflowSpec(teamIDs, tagIDs))

	made, err := a.Actions.Workflows.Create(ctx, workflows.CreateInput{
		Membership: owner, Name: "Sort new conversations by topic", Trigger: "conversation.first_message", Spec: raw,
	})
	if err != nil {
		return fmt.Errorf("create workflow: %w", err)
	}

	if _, err := a.Actions.Workflows.Publish(ctx, workflows.PublishInput{
		Membership: owner, ID: made.ID, AcknowledgeWarnings: true,
	}); err != nil {
		return fmt.Errorf("publish workflow: %w", err)
	}

	return nil
}

/*
marketingWorkflowSpec builds the spec of the topic-sorting workflow,
pointing its steps at the demo's teams and tags.
*/
func marketingWorkflowSpec(teamIDs, tagIDs map[string]ids.UUID) map[string]any {
	step := func(id, kind string, settings map[string]any) map[string]any {
		return map[string]any{"id": id, "kind": kind, "settings": settings}
	}
	team := func(name string) string { return models.PublicID(models.PrefixTeam, teamIDs[name]) }
	tag := func(name string) string { return models.PublicID(models.PrefixTag, tagIDs[name]) }

	return map[string]any{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []any{map[string]any{
			"id": "s_sorttopic1", "kind": "classify",
			"paths": []any{
				map[string]any{
					"id": "p_billing001", "label": "Billing", "description": "Invoices, charges and refunds",
					"steps": []any{
						step("s_tagbill001", "tag", map[string]any{"tag_ids": []string{tag("billing")}}),
						step("s_asgbill001", "assign", map[string]any{"mode": "team", "team_id": team("Billing")}),
					},
				},
				map[string]any{
					"id": "p_bug0000001", "label": "Bug", "description": "Something in the product is broken",
					"steps": []any{
						step("s_tagbug0001", "tag", map[string]any{"tag_ids": []string{tag("bug")}}),
						step("s_priobug001", "priority", map[string]any{"value": "high"}),
						step("s_asgbug0001", "assign", map[string]any{"mode": "team", "team_id": team("Engineering")}),
					},
				},
				map[string]any{"id": "p_other00001", "steps": []any{}, "outcome": "none"},
			},
		}},
	}
}
