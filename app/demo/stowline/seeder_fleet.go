/*
|--------------------------------------------------------------------------
| Seeding The Agent Fleet
|--------------------------------------------------------------------------
|
| Sky answers everyone first. Byte answers the developers' inbox on its
| own, from the developer template. Rio, from the sales template, takes
| plan and cancellation questions, and Juno, from the onboarding
| template, takes setup questions; Sky hands both kinds over to them
| without the customer noticing, so the reports show agents side by
| side and the transfers between them.
|
*/

package stowline

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/ai/agents"
)

/*
fleetAgent is one agent the demo adds beside Sky.
*/
type fleetAgent struct {
	name, template string
	inbox          string
}

/*
fleetAgents are the agents beside Sky, in the order they are made.
*/
func fleetAgents() []fleetAgent {
	return []fleetAgent{
		{name: "Byte", template: "developer", inbox: "Developers"},
		{name: "Rio", template: "sales"},
		{name: "Juno", template: "onboarding"},
	}
}

/*
fleetTopics names the agent that ends up with each topic's
conversations, and whether Sky handed them over first.
*/
func fleetTopics() map[string]fleetRoute {
	return map[string]fleetRoute{
		"api_rate_limit": {agent: "Byte"}, "webhook_signature": {agent: "Byte"}, "api_auth": {agent: "Byte"},
		"plan_question": {agent: "Rio", transferred: true}, "cancel_account": {agent: "Rio", transferred: true},
		"multi_issue": {agent: "Juno", transferred: true}, "how_to_rules": {agent: "Juno", transferred: true},
	}
}

/*
fleetRoute is the agent a topic's conversations end with.
*/
type fleetRoute struct {
	agent       string
	transferred bool
}

/*
fleet makes the agents beside Sky, places Byte on its inbox, lets Sky
hand over to Rio and Juno, and publishes each when the install offers
a model.
*/
func (r *run) fleet(ctx context.Context) error {
	a := r.s.App
	sky, err := a.Actions.AI.Agents.Default(ctx, r.owner)
	if err != nil {
		return fmt.Errorf("find default agent: %w", err)
	}
	r.agents = map[string]ids.UUID{"Sky": sky.Agent.ID}
	for _, f := range fleetAgents() {
		made, err := a.Actions.AI.Agents.Create(ctx, ai.CreateAgentInput{
			Membership: r.owner, Name: f.name, TemplateKey: f.template,
		})
		if err != nil {
			return fmt.Errorf("create agent %s: %w", f.name, err)
		}
		r.agents[f.name] = made.Agent.ID
		patch := ai.AgentPatch{EnabledChannels: &[]string{"widget", "email"}}
		if f.inbox != "" {
			patch.Placement = &agents.Placement{InboxIDs: []ids.UUID{r.inboxes[f.inbox]}}
		}
		if err := r.saveAgent(ctx, made.Agent.ID, patch); err != nil {
			return fmt.Errorf("set up agent %s: %w", f.name, err)
		}
	}
	targets := []ids.UUID{r.agents["Rio"], r.agents["Juno"]}
	if err := r.saveAgent(ctx, sky.Agent.ID, ai.AgentPatch{TransferAgentIDs: &targets}); err != nil {
		return fmt.Errorf("let Sky transfer: %w", err)
	}
	if err := r.simulations(ctx, sky.Agent.ID); err != nil {
		return err
	}
	if !sky.Ready {
		return nil
	}
	for _, name := range []string{"Byte", "Rio", "Juno", "Sky"} {
		if _, err := a.Actions.AI.Agents.Publish(ctx, ai.PublishInput{
			Membership: r.owner, AgentID: r.agents[name], ChangeNote: "Set up by the demo",
		}); err != nil {
			r.s.Log("  %s saved switched off: %v", name, err)
		}
	}
	return nil
}

/*
simulations saves the chats Sky must keep getting right. Each carries
its own scenario, so none is written by a model.
*/
func (r *run) simulations(ctx context.Context, sky ids.UUID) error {
	for _, sim := range Simulations() {
		if _, err := r.s.App.Actions.AI.Simulations.Create(ctx, r.owner, sky, sim.Input(r.actions)); err != nil {
			return fmt.Errorf("save simulation %s: %w", sim.Name, err)
		}
	}
	return nil
}

/*
saveAgent saves a change to the agent's working copy.
*/
func (r *run) saveAgent(ctx context.Context, agent ids.UUID, patch ai.AgentPatch) error {
	_, err := r.s.App.Actions.AI.Agents.Update(ctx, ai.UpdateAgentInput{
		Membership: r.owner, AgentID: agent, Patch: patch,
	})
	return err
}

/*
route picks the agent that answers the conversation, records it to be
written on the conversation, and reports whether Sky hands it over
before the first answer.
*/
func (h *history) route(c Conversation) bool {
	sky := h.run.agents["Sky"]
	rt, ok := fleetTopics()[c.Topic]
	if !ok {
		h.agent = sky
		h.held = append(h.held, []any{c.ID, sky, []ids.UUID{}})
		return false
	}
	h.agent = h.run.agents[rt.agent]
	from := []ids.UUID{}
	if rt.transferred {
		from = append(from, sky)
	}
	h.held = append(h.held, []any{c.ID, h.agent, from})
	return rt.transferred
}

/*
transfer adds Sky's run that handed the conversation to its agent,
just before that agent's first answer.
*/
func (h *history) transfer(c Conversation, k int, trigger ids.UUID) {
	m := c.Messages[k]
	run := stableID(m.At.Add(-20*time.Second), fmt.Sprintf("%s/transfer", c.ID))
	started := m.At.Add(-25 * time.Second)
	cl := call{"generate", SeededModel, 2100 + h.rnd.Int64N(500), 0, 30 + h.rnd.Int64N(20)}
	cost, done := h.price(cl), started.Add(4*time.Second)
	h.steps = append(h.steps, []any{
		stableID(started, fmt.Sprintf("%s/0", run)), h.run.ws.ID, run, 1, cl.kind, "dimmi", cl.model,
		"ok", cl.input, 0, cl.output, cost, 1100 + h.rnd.IntN(900), started, started.Add(time.Second), started,
	})
	h.runs = append(h.runs, []any{
		run, h.run.ws.ID, h.run.agents["Sky"], "conversation", c.ID, trigger, "contact",
		h.run.plan.Contacts[c.Contact].ID, c.Channel, "completed", "transferred", nil, c.Language, c.Subject,
		jsonOf([]map[string]any{}), nil, cl.input, 0, cl.output, cost, 1500 + h.rnd.IntN(900),
		500 + h.rnd.IntN(400), started, done, started, done, nil,
		[]string{},
		nil,
	})
}

/*
holders writes on each conversation the agent that holds it and the
agents it left.
*/
func (h *history) holders(ctx context.Context, tx *database.Tx) error {
	_, err := tx.Exec(ctx, `CREATE TEMP TABLE seed_holders (id uuid PRIMARY KEY, agent uuid, came_from uuid[])
		ON COMMIT DROP`)
	if err != nil {
		return fmt.Errorf("stage holders: %w", err)
	}
	if err := insertMany(ctx, tx, rowSet{"seed_holders", []string{"id", "agent", "came_from"}, h.held}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE conversations c SET ai_agent_id = s.agent, ai_transferred_from_ids = s.came_from
		FROM seed_holders s WHERE c.workspace_id = $1 AND c.id = s.id`, h.run.ws.ID); err != nil {
		return fmt.Errorf("write holders: %w", err)
	}
	return nil
}
