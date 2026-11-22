package costsim_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/costsim"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/demo/stowline"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/tests"
)

/*
tiny is a seed small enough for a test that still has every kind of row.
*/
func tiny() stowline.Size {
	return stowline.Size{Name: "tiny", Companies: 6, Contacts: 40, Conversations: 90, Teammates: 6}
}

/*
seeded boots an application and seeds Stowline into it.
*/
func seeded(t *testing.T) (*tests.TestCase, *stowline.Result) {
	t.Helper()
	tc := tests.New(t)
	res, err := (&stowline.Seeder{App: tc.App, Size: tiny(), Seed: 7}).Run(t.Context())
	require.NoError(t, err)
	return tc, res
}

/*
fingerprint sums up what the seed wrote, ids aside: every message's
author, text and time, in order.
*/
func fingerprint(t *testing.T, tc *tests.TestCase, ws ids.UUID) string {
	t.Helper()
	var out string
	ctx := tenancy.With(t.Context(), ws.String())
	err := tc.Operator().QueryRow(ctx, `SELECT md5(string_agg(m.author_type || m.body || m.created_at::text, '|'
		ORDER BY m.created_at, m.body)) FROM messages m WHERE m.workspace_id = $1`, ws).Scan(&out)
	require.NoError(t, err)
	return out
}

func TestSeed(t *testing.T) {
	t.Parallel()

	t.Run("writes a whole workspace through the real actions and in bulk", func(t *testing.T) {
		t.Parallel()
		_, res := seeded(t)

		c := res.Counts
		assert.GreaterOrEqual(t, c["articles"], int64(150))
		assert.Greater(t, c["knowledge_chunks"], c["articles"], "the help center is indexed for search")
		assert.Equal(t, int64(len(stowline.Collections())), c["collections"])
		assert.Equal(t, int64(tiny().Conversations), c["conversations"])
		assert.Greater(t, c["messages"], int64(tiny().Conversations*2))
		assert.Equal(t, int64(tiny().Contacts), c["contacts"])
		assert.Equal(t, int64(tiny().Companies), c["companies"])
		assert.Equal(t, int64(tiny().Teammates), c["workspace_members"])
		assert.Equal(t, int64(len(stowline.Teams())), c["teams"])
		assert.Positive(t, c["ai_runs"])
		assert.Greater(t, c["ai_run_steps"], c["ai_runs"])
		assert.Equal(t, int64(8), c["ai_actions"])
		assert.Equal(t, int64(3), c["procedures"])
		assert.Equal(t, int64(9), c["procedure_versions"], "each procedure was published three times")
		assert.Equal(t, int64(len(stowline.SavedTests())), c["procedure_test_cases"])
		assert.Positive(t, c["conversation_tag"])
		assert.Positive(t, c["conversation_ratings"])
		assert.Positive(t, c["conversation_events"], "the event log saw every conversation")
		assert.Positive(t, c["changelog_entries"])
		assert.Positive(t, c["feedback_posts"])
	})

	t.Run("gives the procedures a history: dated versions and runs that followed them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		size := stowline.Size{Name: "busy", Companies: 12, Contacts: 80, Conversations: 600, Teammates: 6}
		res, err := (&stowline.Seeder{App: tc.App, Size: size, Seed: 7}).Run(t.Context())
		require.NoError(t, err)
		ctx := tenancy.With(t.Context(), res.Workspace.ID.String())

		var versions, dated int
		require.NoError(t, tc.Operator().QueryRow(ctx, `SELECT count(*),
				count(*) FILTER (WHERE published_at < now() - interval '10 days')
			FROM procedure_versions WHERE workspace_id = $1`, res.Workspace.ID).Scan(&versions, &dated))
		assert.Equal(t, 9, versions)
		assert.Equal(t, versions, dated, "every version is dated in the past")

		var followed, traced, live int
		require.NoError(t, tc.Operator().QueryRow(ctx, `SELECT count(*) FILTER (WHERE flags <> '{}'),
				count(*) FILTER (WHERE procedure_trace ? 'steps'),
				count(*) FILTER (WHERE (procedure_trace->>'version')::int = 3)
			FROM ai_runs WHERE workspace_id = $1`, res.Workspace.ID).Scan(&followed, &traced, &live))
		assert.Positive(t, followed, "runs on a procedure's topic follow it")
		assert.Equal(t, followed, traced, "a run that follows a procedure carries its trace")
		assert.Positive(t, live, "recent runs follow the live version")
		assert.Positive(t, res.Counts["ai_action_calls"], "action steps record their calls")
	})

	t.Run("refuses an existing workspace, and a reset writes the same data again", func(t *testing.T) {
		t.Parallel()
		tc, first := seeded(t)
		before := fingerprint(t, tc, first.Workspace.ID)

		_, err := (&stowline.Seeder{App: tc.App, Size: tiny(), Seed: 7}).Run(t.Context())
		require.ErrorIs(t, err, stowline.ErrExists)

		again, err := (&stowline.Seeder{App: tc.App, Size: tiny(), Seed: 7, Reset: true}).Run(t.Context())
		require.NoError(t, err)
		assert.NotEqual(t, first.Workspace.ID, again.Workspace.ID)
		assert.Equal(t, before, fingerprint(t, tc, again.Workspace.ID), "the same seed writes the same messages")

		_, found, err := stowline.Find(t.Context(), tc.App)
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("never runs in production", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.App.Application.Config.App.Env = foundation.EnvProduction
		_, err := (&stowline.Seeder{App: tc.App, Size: tiny(), Seed: 7}).Run(t.Context())
		require.ErrorIs(t, err, stowline.ErrProduction)
	})
}

/*
scripted is a fake agent that searches once and then answers every
call, with usage priced on the agent's model.
*/
func scripted() *fake.LLM {
	usage := contracts.LLMUsage{InputTokens: 2_000, CachedInputTokens: 6_000, OutputTokens: 150, Reported: true}
	return fake.New(contracts.LLMProviderDimmi).Script(
		fake.Step{ToolCalls: []contracts.ToolCall{{
			ID: "call_search", Name: "search_knowledge",
			Arguments: []byte(`{"query":"void refund USPS"}`),
		}}, Usage: usage},
		fake.Step{
			Text:  "Voided labels are refunded to your Stowline balance once the carrier confirms [1].",
			Usage: usage,
		},
	)
}

/*
simulate runs the harness against the seeded workspace in a dry run on
the fake.
*/
func simulate(t *testing.T, tc *tests.TestCase, res *stowline.Result, change func(*costsim.Options)) *costsim.Report {
	t.Helper()
	ctx := tenancy.With(t.Context(), res.Workspace.ID.String())
	var inbox, channel ids.UUID
	require.NoError(t, tc.Operator().QueryRow(ctx, `SELECT i.id, c.id FROM inboxes i JOIN channel_connections c
		ON c.inbox_id = i.id AND c.type = 'widget' WHERE i.workspace_id = $1 AND i.name = 'Support'`,
		res.Workspace.ID).Scan(&inbox, &channel))

	o := costsim.Options{
		Owner: res.Owner, Inbox: inbox, Channel: channel, Company: stowline.Name, Scenarios: stowline.Scenarios(),
		Conversations: 3, Mode: costsim.ModeScripted, AgentModel: stowline.SeededModel, MaxTurns: 4, DryRun: true,
		Actions: stowline.StubAPI, Model: scripted(), Label: "test",
	}
	if change != nil {
		change(&o)
	}
	h := &costsim.Harness{App: tc.App, Budget: costsim.NewBudget(tc.App.AI.Prices, o.MaxSpend, o.MaxPerConv)}
	report, err := h.Run(context.WithoutCancel(t.Context()), o)
	require.NoError(t, err)
	return report
}

func TestSimulation(t *testing.T) {
	t.Parallel()

	t.Run("puts the agent's model back after the run", func(t *testing.T) {
		t.Parallel()
		tc, res := seeded(t)
		model := func() string {
			var m *string
			require.NoError(t, tc.Operator().QueryRow(t.Context(), `SELECT answer_model FROM ai_agents
				WHERE workspace_id = $1`, res.Workspace.ID).Scan(&m))
			if m == nil {
				return ""
			}
			return *m
		}
		before := model()
		simulate(t, tc, res, func(o *costsim.Options) { o.AgentModel = "glm-5.3-flash" })
		assert.Equal(t, before, model())
	})

	t.Run("plays scripted customers through the real pipeline and reads the cost back", func(t *testing.T) {
		t.Parallel()
		tc, res := seeded(t)
		report := simulate(t, tc, res, nil)

		require.Len(t, report.Conversations, 3)
		first := report.Conversations[0]
		assert.Equal(t, stowline.Scenarios()[0].Key, first.Scenario)
		assert.Positive(t, first.AIReplies)
		assert.Equal(t, 1, first.Searches, "the agent searched the help center")
		assert.GreaterOrEqual(t, first.ModelCalls, 2)
		assert.Positive(t, first.CostMicros, "the recorded runs carry a cost")
		assert.Positive(t, first.ConvCost)
		assert.LessOrEqual(t, first.ConvCost, first.CostMicros,
			"the conversation's own total leaves out runs after a handoff; the runs hold the whole cost")
		assert.Positive(t, first.CachedTokens)
		assert.Equal(t, report.SpendMicros, report.Overall.PerConversation.Total,
			"the guard counted what the runs recorded")
		assert.InDelta(t, 0.75, report.Overall.CacheHitShare, 0.01)

		disk, err := local.New(local.Options{Root: t.TempDir()})
		require.NoError(t, err)
		js, md, err := report.Write(t.Context(), disk, "run")
		require.NoError(t, err)
		for _, file := range []string{js, md} {
			found, err := disk.Exists(t.Context(), file)
			require.NoError(t, err)
			assert.True(t, found, file)
		}
		assert.Contains(t, report.Markdown(), "By scenario kind")

		require.NotNil(t, first.Layer, "the seeded agent runs with the tool layer on")
		assert.Positive(t, first.Layer.FixedTokens)
		assert.Positive(t, first.Layer.SystemTokens)
		assert.NotEmpty(t, first.Layer.Steps)
		assert.Contains(t, report.Markdown(), "## Tool layer")
		assert.Positive(t, first.Ledger["fixed"].Tokens(), "the ledger splits the input by part")
		assert.Positive(t, first.Ledger["newest"].Tokens())
		assert.Positive(t, first.Ledger["output"].Tokens())
		assert.Contains(t, report.Markdown(), "## Token ledger")
	})

	t.Run("stops cleanly when the run's spend limit is reached", func(t *testing.T) {
		t.Parallel()
		tc, res := seeded(t)
		report := simulate(t, tc, res, func(o *costsim.Options) {
			o.Conversations, o.MaxSpend = 10, 1_500
		})

		assert.Equal(t, "run spend limit", report.StoppedBy)
		assert.Less(t, len(report.Conversations), 10)
		assert.LessOrEqual(t, report.SpendMicros, int64(1_500))
	})

	t.Run("holds each conversation to its turn limit", func(t *testing.T) {
		t.Parallel()
		tc, res := seeded(t)
		report := simulate(t, tc, res, func(o *costsim.Options) {
			o.Scenarios = []costsim.Scenario{scenarioOf(costsim.KindTroubleshooting)}
			o.Conversations, o.MaxTurns = 1, 2
		})

		require.Len(t, report.Conversations, 1)
		assert.Equal(t, costsim.KindTroubleshooting, report.Conversations[0].Kind)
		assert.LessOrEqual(t, report.Conversations[0].Turns, 2)
	})

	t.Run("measures Operator on teammates' requests", func(t *testing.T) {
		t.Parallel()
		tc, res := seeded(t)
		h := &costsim.Harness{App: tc.App, Budget: costsim.NewBudget(tc.App.AI.Prices, 0, 0)}
		report, err := h.RunOperator(t.Context(), costsim.Options{
			Owner: res.Owner, Conversations: 2, AgentModel: stowline.SeededModel, DryRun: true,
			Model: fake.New(contracts.LLMProviderDimmi).Script(fake.Step{
				Text:  "There are 4 open conversations.",
				Usage: contracts.LLMUsage{InputTokens: 3_000, OutputTokens: 40, Reported: true},
			}),
		}, stowline.TeammateRequests(), []authz.Membership{res.Owner})
		require.NoError(t, err)

		require.Len(t, report.Conversations, 2)
		assert.Equal(t, "answered", report.Conversations[0].Outcome, report.Conversations[0].Error)
		assert.Positive(t, report.Conversations[0].CostMicros)
		require.NotNil(t, report.Conversations[0].Layer, "Operator runs with the tool layer on")
		assert.Positive(t, report.Conversations[0].Layer.SystemTokens)
	})
}

/*
scenarioOf returns the first scenario of the kind.
*/
func scenarioOf(kind string) costsim.Scenario {
	for _, sc := range stowline.Scenarios() {
		if sc.Kind == kind {
			return sc
		}
	}
	return costsim.Scenario{}
}
