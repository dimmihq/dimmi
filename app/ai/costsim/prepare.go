/*
|--------------------------------------------------------------------------
| Preparing A Run
|--------------------------------------------------------------------------
|
| Before the first conversation the harness points the agent at the
| models asked for, puts the spend guard around every driver its
| connections build, or the offline model in a dry run, answers the
| actions' API in process, keeps every step's request and response, and
| stops the answer job from running beside it, since the harness runs
| each turn itself. Everything it changes is put back afterwards.
|
*/

package costsim

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
prepared is the agent a run uses and what it changed on the
application, to put back after.
*/
type prepared struct {
	agent    models.AiAgent
	restores []func()
}

/*
restore puts back everything the run changed, last change first.
*/
func (p *prepared) restore() {
	for _, undo := range slices.Backward(p.restores) {
		undo()
	}
}

/*
prepare readies the application for the run.
*/
func (h *Harness) prepare(ctx context.Context, o *Options) (*prepared, error) {
	a := h.App
	if a.AI == nil {
		return nil, errors.New("the agent's runner is not booted")
	}
	p := &prepared{}
	if err := h.within(ctx, func(ctx context.Context) error { return h.pointAgent(ctx, o, p) }); err != nil {
		p.restore()
		return nil, err
	}
	h.provider = h.answerKey(ctx, p.agent).Provider
	if o.DryRun {
		h.offline(ctx, o, p)
	} else if err := h.meter(ctx, p); err != nil {
		p.restore()
		return nil, err
	}
	if o.Actions != nil {
		if err := h.answerActions(o, p); err != nil {
			p.restore()
			return nil, err
		}
	}

	enabled, payloads := a.Config.AI.Enabled, a.AI.StorePayloads
	a.Config.AI.Enabled, a.AI.StorePayloads = false, true
	p.restores = append(p.restores, func() { a.Config.AI.Enabled, a.AI.StorePayloads = enabled, payloads })
	return p, nil
}

/*
pointAgent loads the agent and points it at the models asked for,
giving it a stand-in connection in a dry run when it has none.
*/
func (h *Harness) pointAgent(ctx context.Context, o *Options, p *prepared) error {
	q := models.New(h.App.DB)
	agent, err := q.FindAgent(ctx, o.Owner.Workspace.ID)
	if err != nil {
		return fmt.Errorf("find agent: %w", err)
	}
	p.agent = agent
	if agent.AnswerKeyID == nil && o.DryRun {
		if err := h.dryKey(ctx, p); err != nil {
			return err
		}
	}
	if p.agent.AnswerKeyID == nil {
		return ErrNoAgentModel
	}
	original := p.agent
	if o.AnswerKey != nil {
		if _, err := h.App.DB.Exec(ctx, `UPDATE ai_agents SET answer_key_id = $2 WHERE id = $1`, original.ID,
			*o.AnswerKey); err != nil {
			return fmt.Errorf("point agent at connection: %w", err)
		}
	}
	if _, err := h.App.DB.Exec(ctx, `UPDATE ai_agents SET answer_model = COALESCE(NULLIF($2, ''), answer_model),
			classifier_model = COALESCE(NULLIF($3, ''), classifier_model), enabled = true,
			enabled_channels = CASE WHEN 'widget' = ANY(enabled_channels) THEN enabled_channels
				ELSE enabled_channels || '{widget}' END
		WHERE id = $1`, original.ID, o.AgentModel, o.ClassifierModel); err != nil {
		return fmt.Errorf("point agent at models: %w", err)
	}
	p.restores = append(p.restores, h.later(ctx, func(ctx context.Context) {
		_, _ = h.App.DB.Exec(ctx, `UPDATE ai_agents SET answer_model = $2, classifier_model = $3, enabled = $4,
			enabled_channels = $5, answer_key_id = $6 WHERE id = $1`, original.ID, original.AnswerModel,
			original.ClassifierModel, original.Enabled, original.EnabledChannels, original.AnswerKeyID)
	}))
	p.agent, err = q.FindAgent(ctx, o.Owner.Workspace.ID)
	return err
}

/*
dryKey gives an agent without a connection a stand-in one for a dry
run, removed again afterwards, so a dry run works on an install that
offers no model.
*/
func (h *Harness) dryKey(ctx context.Context, p *prepared) error {
	a, agent := h.App, p.agent
	var key ids.UUID
	if err := a.DB.QueryRow(ctx, `INSERT INTO ai_provider_keys (workspace_id, name, provider, auth_type, status, source)
		VALUES ($1, 'dry run', 'dimmi', 'none', 'valid', 'system') RETURNING id`,
		agent.WorkspaceID).Scan(&key); err != nil {
		return fmt.Errorf("add dry run connection: %w", err)
	}
	model := strv(agent.AnswerModel)
	if model == "" {
		model = "gpt-6-luna"
	}
	if _, err := a.DB.Exec(ctx, `UPDATE ai_agents SET answer_key_id = $2, answer_model = $3 WHERE id = $1`,
		agent.ID, key, model); err != nil {
		return fmt.Errorf("point agent at dry run connection: %w", err)
	}
	p.restores = append(p.restores, h.later(ctx, func(ctx context.Context) {
		_, _ = a.DB.Exec(ctx, `UPDATE ai_agents SET answer_key_id = $2, answer_model = $3 WHERE id = $1`,
			agent.ID, agent.AnswerKeyID, agent.AnswerModel)
		_, _ = a.DB.Exec(ctx, `UPDATE ai_provider_keys SET deleted_at = now() WHERE id = $1`, key)
	}))
	var err error
	p.agent, err = models.New(a.DB).FindAgent(ctx, agent.WorkspaceID)
	return err
}

/*
offline makes every provider's driver the offline model, or the model
the options name, behind the spend guard, so nothing a connection
names can leave the process.
*/
func (h *Harness) offline(ctx context.Context, o *Options, p *prepared) {
	h.App.LLM.Flush()
	p.restores = append(p.restores, h.App.LLM.Flush)
	providers := []string{
		string(enums.AIProviderDimmi), string(enums.AIProviderAnthropic), string(enums.AIProviderOpenAI),
		string(enums.AIProviderChatGPT), string(enums.AIProviderEngy), string(enums.AIProviderBaseten),
		string(enums.AIProviderOpenAICompatible),
	}
	for _, k := range h.keyProviders(ctx, p.agent) {
		if !contains(providers, k) {
			providers = append(providers, k)
		}
	}
	for _, provider := range providers {
		model := o.Model
		if model == nil {
			model = NewOffline(contracts.LLMProvider(provider))
		}
		h.App.LLM.Extend(enums.AIProvider(provider), func(_ context.Context, _ *managers.LLMManager,
			key models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
		) (contracts.LLM, error) {
			return Guard(model, h.Budget, key.Provider), nil
		})
	}
}

/*
meter puts the spend guard around every driver the agent's connections
build.
*/
func (h *Harness) meter(ctx context.Context, p *prepared) error {
	h.App.LLM.Flush()
	p.restores = append(p.restores, h.App.LLM.Flush)
	for _, provider := range h.keyProviders(ctx, p.agent) {
		build, ok := h.App.LLM.Factory(enums.AIProvider(provider))
		if !ok {
			return fmt.Errorf("no driver for provider %q", provider)
		}
		h.App.LLM.Extend(enums.AIProvider(provider), func(ctx context.Context, m *managers.LLMManager,
			key models.AiProviderKey, creds managers.Credentials, hc *httpclient.Client,
		) (contracts.LLM, error) {
			drv, err := build(ctx, m, key, creds, hc)
			if err != nil {
				return nil, err
			}
			return Guard(drv, h.Budget, key.Provider), nil
		})
	}
	return nil
}

/*
answerActions answers the actions' API in process, on a client the
action kits use for the run.
*/
func (h *Harness) answerActions(o *Options, p *prepared) error {
	client, err := httpclient.New(httpclient.Config{AllowFakes: true, Timeout: 10 * time.Second})
	if err != nil {
		return err
	}
	o.Actions(client)
	for _, kit := range kitsOf(h.App) {
		before := kit.HTTP
		kit.HTTP = client
		p.restores = append(p.restores, func() { kit.HTTP = before })
	}
	return nil
}

/*
keyProviders lists the providers of the agent's connections.
*/
func (h *Harness) keyProviders(ctx context.Context, agent models.AiAgent) []string {
	var out []string
	_ = h.within(ctx, func(ctx context.Context) error {
		for _, id := range []*ids.UUID{agent.AnswerKeyID, agent.ClassifierKeyID, agent.FallbackKeyID} {
			if id == nil {
				continue
			}
			key, err := models.New(h.App.DB).FindProviderKey(ctx, models.FindProviderKeyParams{
				WorkspaceID: agent.WorkspaceID, ID: *id,
			})
			if err == nil && !contains(out, key.Provider) {
				out = append(out, key.Provider)
			}
		}
		return nil
	})
	return out
}

/*
answerKey is the connection the agent answers on, which the
model-played customer runs on too.
*/
func (h *Harness) answerKey(ctx context.Context, agent models.AiAgent) models.AiProviderKey {
	var key models.AiProviderKey
	_ = h.within(ctx, func(ctx context.Context) error {
		var err error
		key, err = models.New(h.App.DB).FindProviderKey(ctx, models.FindProviderKeyParams{
			WorkspaceID: agent.WorkspaceID, ID: *agent.AnswerKeyID,
		})
		return err
	})
	return key
}

/*
within runs the function in a transaction inside the workspace the
context names, so row-level security sees the tenant.
*/
func (h *Harness) within(ctx context.Context, fn func(ctx context.Context) error) error {
	return h.App.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error { return fn(ctx) })
}

/*
later returns a function that runs fn in the workspace after the run,
even when the run's context was canceled.
*/
func (h *Harness) later(ctx context.Context, fn func(ctx context.Context)) func() {
	return func() {
		_ = h.within(context.WithoutCancel(ctx), func(ctx context.Context) error {
			fn(ctx)
			return nil
		})
	}
}

/*
kitsOf lists the action kits the agent and the dashboard call actions
through.
*/
func kitsOf(a *app.App) []*actionkit.Kit {
	var out []*actionkit.Kit
	if a.AI != nil && a.AI.Actions != nil {
		out = append(out, a.AI.Actions)
	}
	if a.Actions.AI != nil && a.Actions.AI.ActionKit != nil && (a.AI == nil || a.Actions.AI.ActionKit != a.AI.Actions) {
		out = append(out, a.Actions.AI.ActionKit)
	}
	return out
}
