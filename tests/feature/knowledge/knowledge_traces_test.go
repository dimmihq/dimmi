package knowledge_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/fake"
	"github.com/dimmihq/dimmi/tests"
)

/*
A search that falls back keeps its answer and is still a failure on the
tracker: the search tool's span fails with the cause (services/span.rs
counts a span whose status is set and not "ok"), says it recovered and
how, the run around it is flagged degraded but not failed, and one
issue per cause is reported, tied to the trace, with no query in it.
*/
func TestKnowledgeFallbackTraces(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		breaks   func(x index)
		options  contracts.KnowledgeSearchOptions
		kind     string
		status   string
		fallback string
	}{
		{
			name:   "the embedder fails after its retry",
			breaks: func(x index) { x.embed.Fail(serverDown()) },
			kind:   "knowledge_embedding_error", status: "internal_error", fallback: "keywords_only",
		},
		{
			name:    "the embedder times out",
			breaks:  func(x index) { x.embed.Hang() },
			options: contracts.KnowledgeSearchOptions{QueryEmbedTimeout: 20 * time.Millisecond},
			kind:    "knowledge_embedding_timeout", status: "deadline_exceeded", fallback: "keywords_only",
		},
		{
			name:   "the reranker fails",
			breaks: func(x index) { x.rerank.fail(serverDown()) },
			kind:   "knowledge_rerank_failed", status: "internal_error", fallback: "unranked",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tracker := tests.NewTracker(t)
			x := boot(t, tests.WithEnv("DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY=test-key"), tracker.Option())
			x.embed.Script("refund", fake.Axis(0, 1024))
			x.publish(t, "Refund policy", "A refund takes five business days.")
			x.publish(t, "Refund exceptions", "A refund is never paid on custom pallets.")
			x.tc.Drain()
			tc.breaks(x)

			res := x.searchIn(t, "how long does a refund take", tc.options)
			assert.NotEmpty(t, res.Chunks, "the search still answers")

			var tool, run tests.TrackedSpan
			for _, s := range x.tc.TrackedSpans(tracker) {
				switch s.Kind() {
				case "tool":
					tool = s
				case "agent":
					run = s
				}
			}
			assert.Equal(t, tc.status, tool.Status)
			assert.Equal(t, tc.kind, tool.Attr("error.type"))
			assert.Equal(t, true, tool.Data["error.recovered"])
			assert.Equal(t, tc.fallback, tool.Attr("fallback"))
			assert.Empty(t, run.Status, "the run recovered")
			assert.Equal(t, true, run.Data["degraded"])
			assert.Contains(t, run.Attr("degraded.kinds"), tc.kind)

			var found []tests.TrackedIssue
			for _, issue := range x.tc.TrackedIssues(tracker) {
				if issue.Tags["error.type"] == tc.kind {
					found = append(found, issue)
				}
			}
			require.NotEmpty(t, found)
			assert.Equal(t, []string{"recovered", tc.kind}, found[0].Fingerprint)
			assert.Equal(t, tool.TraceID, found[0].Contexts.Trace.TraceID)
			assert.NotContains(t, found[0].Raw, "refund", "no query leaves in the issue")
		})
	}
}

/*
searchIn searches the way the agent's search tool does: inside an agent
run and a tool call.
*/
func (x index) searchIn(t *testing.T, query string, opts contracts.KnowledgeSearchOptions,
) contracts.KnowledgeSearchResult {
	t.Helper()
	tel := x.tc.App.Telemetry
	ctx, run := tel.StartAI(t.Context(), telemetry.OpAgent, "invoke_agent Ada")
	run.Set("gen_ai.agent.name", "Ada")
	toolCtx, tool := tel.StartAI(ctx, telemetry.OpTool, "execute_tool search_knowledge")
	tool.Set("gen_ai.tool.name", "search_knowledge")
	opts.Visibility = []contracts.KnowledgeVisibility{"public", "agent_only"}
	res, err := x.tc.App.Knowledge.Search(context.WithoutCancel(toolCtx), x.tc.Workspace.Workspace.ID, query, opts)
	require.NoError(t, err)
	tool.End()
	run.End()
	return res
}

/*
serverDown is a provider failure on its side.
*/
func serverDown() error {
	return &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "down"}
}
