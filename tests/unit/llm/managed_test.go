package llm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/drivers/llm/managed"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/aimodels"
)

func TestManagedDriver(t *testing.T) {
	t.Parallel()

	t.Run("routes a model to its family's upstream", func(t *testing.T) {
		t.Parallel()
		claude := fake.New(contracts.LLMProviderAnthropic).Script(fake.Step{Text: "from claude"})
		families := make(map[enums.AIModelFamily]contracts.LLM)
		families[enums.AIModelFamilyAnthropic] = claude
		drv := managed.New(families, nil)

		res, err := drv.Chat(t.Context(), request("claude-sonnet-5-5"))

		require.NoError(t, err)
		assert.Equal(t, "from claude", res.Text)
		assert.Equal(t, contracts.LLMProviderDimmi, drv.Provider())
	})

	t.Run("refuses a model whose family has no upstream", func(t *testing.T) {
		t.Parallel()
		glm := fake.New(contracts.LLMProviderEngy)
		families := make(map[enums.AIModelFamily]contracts.LLM)
		families[enums.AIModelFamilyOpenWeight] = glm
		drv := managed.New(families, nil)

		_, err := drv.Stream(t.Context(), request("claude-sonnet-5-5"))

		assert.Equal(t, contracts.LLMErrorModelNotFound, llmError(t, err).Kind)
		assert.Empty(t, glm.Sent())
		assert.Zero(t, drv.Capabilities("claude-sonnet-5-5").ContextTokens)
	})

	t.Run("offers only the models of configured families", func(t *testing.T) {
		t.Parallel()

		offered := aimodels.ManagedFor(func(f enums.AIModelFamily) bool { return f == enums.AIModelFamilyOpenWeight })

		require.Len(t, offered, 1)
		assert.Equal(t, "glm-5.3-flash", offered[0].ID)
		assert.NotNil(t, aimodels.Find(aimodels.Anthropic(), "claude-fable-5-1"))
	})
}
