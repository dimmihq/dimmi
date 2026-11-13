//go:build dev

/*
|--------------------------------------------------------------------------
| AI Simulate
|--------------------------------------------------------------------------
|
| Measures what AI conversations really cost: plays simulated customers
| from hand-written Stowline scenarios against the real customer-facing
| agent in the Stowline demo workspace, or teammates' requests against
| Operator, and writes a report of each conversation's cost, read from
| the runs the app recorded, under tmp/.
|
| Real model calls cost money, so a real run needs a spend limit and
| --confirm; without --confirm it only prints the models, the limits and
| a rough estimate. --dry-run runs everything on an offline model and
| spends nothing.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/ai/costsim"
	"github.com/dimmihq/dimmi/app/mcpserver"
	"github.com/dimmihq/dimmi/app/demo/stowline"
)

/*
AISimulateCommand runs the AI cost simulation.
*/
type AISimulateCommand struct {
	App *app.App
}

/*
Name returns ai:simulate.
*/
func (c AISimulateCommand) Name() string {
	return "ai:simulate"
}

/*
Description says what the command does and that it can spend money.
*/
func (c AISimulateCommand) Description() string {
	return fmt.Sprintf("Play simulated customers (%d hand-written scenarios) against the real AI agent in the "+
		"Stowline demo workspace and report what each conversation cost, read from the recorded runs, under "+
		"tmp/costsim. Real runs call paid models: they need --max-spend and --confirm, and print an estimate "+
		"first. --dry-run uses an offline model and spends nothing. --agent operator measures Operator instead",
		len(stowline.Scenarios()))
}

/*
Flags sets the size of the run, the models, the limits and the mode.
*/
func (c AISimulateCommand) Flags(flags *console.Flags) {
	flags.Int("conversations", 100, "how many conversations to simulate")
	flags.String("agent", "dimmi", "which agent to measure: dimmi, the customer-facing agent, or operator")
	flags.String("mode", costsim.ModeScripted, "how the customer is played: scripted (no model) or model")
	flags.String("agent-model", "", "the model the agent answers with; empty keeps the agent's own")
	flags.String("classifier-model", "", "the model the agent classifies with; empty keeps the agent's own")
	flags.String("customer-model", "gpt-6-luna", "the model that plays the customer in model mode")
	flags.String("max-spend", "", "required for a real run: stop before the whole run would spend more, in dollars")
	flags.String("max-conversation-spend", "0.50", "stop a conversation before it would spend more, in dollars")
	flags.Int("max-turns", 15, "stop a conversation after this many customer messages")
	flags.Bool("dry-run", "run everything on an offline model: no real model calls, no spend")
	flags.Bool("confirm", "start a real run after printing the estimate")
	flags.String("out", "tmp/costsim", "the directory the report is written under")
	flags.String("scenarios", "", "play only these scenarios, by key, comma separated; empty plays them all in turn")
}

/*
Handle prints the plan and, for a dry run or a confirmed real run,
runs it and writes the report.
*/
func (c AISimulateCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	o, err := c.options(ctx, in, out)
	if err != nil {
		return err
	}
	budget := costsim.NewBudget(c.App.AI.Prices, o.MaxSpend, o.MaxPerConv)
	provider, err := c.agentModel(ctx, &o)
	if err != nil {
		return err
	}
	c.plan(out, o, provider, costsim.Estimate(budget, provider, o))
	if !o.DryRun && !in.Bool("confirm") {
		out.Warn("Nothing was run. This calls paid models; run again with --confirm to start.")
		return nil
	}

	report, err := c.run(ctx, in.String("agent"), &costsim.Harness{App: c.App, Budget: budget}, o)
	if err != nil {
		return err
	}
	disk, err := c.App.Storage.Build("costsim-report", config.Section{"driver": "local", "root": in.String("out")})
	if err != nil {
		return fmt.Errorf("open report directory: %w", err)
	}
	js, md, err := report.Write(ctx, disk, o.Label)
	if err != nil {
		return err
	}
	g := report.Overall
	out.Info("%d conversations: mean %.3f¢, median %.3f¢, p95 %.3f¢, max %.3f¢ per conversation; "+
		"%.3f¢ per AI reply; cache hit %.0f%%. Guarded spend $%.4f.", len(report.Conversations),
		g.PerConversation.Mean/1e4, float64(g.PerConversation.Median)/1e4, float64(g.PerConversation.P95)/1e4,
		float64(g.PerConversation.Max)/1e4, g.PerReply.Mean/1e4, g.CacheHitShare*100,
		float64(report.SpendMicros)/1e6)
	out.Line("Report: %s/%s and %s/%s", in.String("out"), js, in.String("out"), md)
	return nil
}

/*
options reads the flags into the run's options.
*/
func (c AISimulateCommand) options(ctx context.Context, in *console.Input, out *console.Output,
) (costsim.Options, error) {
	if c.App.AI == nil {
		return costsim.Options{}, errors.New("the AI runner is not booted")
	}
	dry := in.Bool("dry-run")
	maxSpend, err := dollars(in.String("max-spend"))
	if err != nil {
		return costsim.Options{}, fmt.Errorf("--max-spend: %w", err)
	}
	if !dry && maxSpend <= 0 {
		return costsim.Options{}, errors.New("a real run needs --max-spend in dollars, such as --max-spend 5; " +
			"or use --dry-run")
	}
	perConv, err := dollars(in.String("max-conversation-spend"))
	if err != nil {
		return costsim.Options{}, fmt.Errorf("--max-conversation-spend: %w", err)
	}
	mode := in.String("mode")
	if mode != costsim.ModeScripted && mode != costsim.ModeModel {
		return costsim.Options{}, fmt.Errorf("--mode is %s or %s", costsim.ModeScripted, costsim.ModeModel)
	}
	scenarios, err := pickScenarios(in.String("scenarios"))
	if err != nil {
		return costsim.Options{}, err
	}
	place, err := c.demo(ctx)
	if err != nil {
		return costsim.Options{}, err
	}
	label := c.App.Clock.Now().UTC().Format("20060102-150405")
	if dry {
		label += "-dry"
	}
	return costsim.Options{
		Owner: place.Owner, Inbox: place.Inbox, Channel: place.Channel, Company: stowline.Name,
		Scenarios: scenarios, Conversations: in.Int("conversations"), Mode: mode,
		AgentModel: in.String("agent-model"), ClassifierModel: in.String("classifier-model"),
		CustomerModel: in.String("customer-model"), MaxSpend: maxSpend, MaxPerConv: perConv,
		MaxTurns: in.Int("max-turns"), DryRun: dry, Actions: stowline.StubAPI, Label: label, Log: out.Line,
	}, nil
}

/*
plan prints what the run would do and roughly what it would cost.
*/
func (c AISimulateCommand) plan(out *console.Output, o costsim.Options, provider string, estimate int64) {
	out.Line("Agent:          %s (connection provider %q)", o.AgentModel, provider)
	if o.Mode == costsim.ModeModel {
		out.Line("Customer:       played by %s", o.CustomerModel)
	} else {
		out.Line("Customer:       scripted, no model calls")
	}
	out.Line("Conversations:  %d over %d scenarios", o.Conversations, len(o.Scenarios))
	out.Line("Limits:         run $%.2f, conversation $%.2f, %d customer messages", float64(o.MaxSpend)/1e6,
		float64(o.MaxPerConv)/1e6, o.MaxTurns)
	out.Line("Rough estimate: $%.2f (scripted turns, about 3 model calls a turn)", float64(estimate)/1e6)
}

/*
run runs the simulation on the agent the flag names.
*/
func (c AISimulateCommand) run(ctx context.Context, agent string, h *costsim.Harness, o costsim.Options,
) (*costsim.Report, error) {
	if agent != "operator" {
		return h.Run(ctx, o)
	}
	team, err := DemoTeam(ctx, c.App, o.Owner)
	if err != nil {
		return nil, err
	}
	report, err := h.RunOperator(ctx, o, stowline.TeammateRequests(), team)
	if err != nil {
		return nil, err
	}
	if lister, ok := c.App.Actions.OperatorAgent.Engine.Catalog.(interface{ Left() []mcpserver.Left }); ok {
		for _, l := range lister.Left() {
			report.LeftOut = append(report.LeftOut, costsim.LeftOut{Name: l.Name, Reason: l.Reason})
		}
	}
	return report, nil
}

/*
pickScenarios returns the scenarios the keys name, in their order, or
every scenario when none are named.
*/
func pickScenarios(keys string) ([]costsim.Scenario, error) {
	all := stowline.Scenarios()
	if keys == "" {
		return all, nil
	}
	var out []costsim.Scenario
	for key := range strings.SplitSeq(keys, ",") {
		i := slices.IndexFunc(all, func(s costsim.Scenario) bool { return s.Key == strings.TrimSpace(key) })
		if i < 0 {
			return nil, fmt.Errorf("--scenarios: no scenario %q", key)
		}
		out = append(out, all[i])
	}
	return out, nil
}

/*
dollars reads an amount of dollars as micro dollars; empty is zero.
*/
func dollars(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, errors.New("an amount in dollars, such as 5 or 0.50")
	}
	return int64(f * 1e6), nil
}

/*
agentModel fills in the agent's own answer model when none was asked
for and returns its connection's provider. A dry run on an agent with
no connection runs on the default model.
*/
func (c AISimulateCommand) agentModel(ctx context.Context, o *costsim.Options) (string, error) {
	var model, provider *string
	err := c.App.DB.Transaction(tenancy.With(ctx, o.Owner.Workspace.ID.String()),
		func(ctx context.Context, tx *database.Tx) error {
			return tx.QueryRow(ctx, `SELECT a.answer_model, k.provider FROM ai_agents a
				LEFT JOIN ai_provider_keys k ON k.id = a.answer_key_id AND k.deleted_at IS NULL
				WHERE a.workspace_id = $1 AND a.deleted_at IS NULL`, o.Owner.Workspace.ID).Scan(&model, &provider)
		})
	if err != nil {
		return "", fmt.Errorf("read agent: %w", err)
	}
	if provider == nil {
		if !o.DryRun {
			return "", costsim.ErrNoAgentModel
		}
		model, provider = new(stowline.SeededModel), new("dimmi")
	}
	if o.AgentModel == "" && model != nil {
		o.AgentModel = *model
	}
	return *provider, nil
}
