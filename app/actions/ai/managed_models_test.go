package ai

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/config"
)

/*
familiesOf lists the families of the models, each once, in order.
*/
func familiesOf(entries []aimodels.Entry) []enums.AIModelFamily {
	var out []enums.AIModelFamily

	for _, e := range entries {
		if !slices.Contains(out, e.Family) {
			out = append(out, e.Family)
		}
	}

	slices.Sort(out)

	return out
}

func TestManagedModels(t *testing.T) {
	t.Parallel()

	held := config.AIManagedUpstream{APIKey: "set"}
	hosts := config.AIManagedOpenWeight{Engy: held}

	t.Run("offers nothing when the install holds no key", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, managedModels(config.AIConfig{}))
		assert.Empty(t, managedModelIDs(config.AIConfig{}))
		assert.Empty(t, catalog(nil, config.AIConfig{})[string(enums.AIProviderDimmi)])
	})

	t.Run("offers only the families the install holds a key for", func(t *testing.T) {
		t.Parallel()

		cfg := config.AIConfig{Managed: config.AIManagedConfig{Anthropic: held}}

		assert.Equal(t, []enums.AIModelFamily{enums.AIModelFamilyAnthropic}, familiesOf(managedModels(cfg)))
		assert.Len(t, catalog(nil, cfg)[string(enums.AIProviderDimmi)], len(managedModels(cfg)))
		assert.Len(t, managedModelIDs(cfg), len(managedModels(cfg)))

		cfg = config.AIConfig{Managed: config.AIManagedConfig{OpenAI: held, OpenWeight: hosts}}

		assert.Equal(t,
			[]enums.AIModelFamily{enums.AIModelFamilyOpenAI, enums.AIModelFamilyOpenWeight},
			familiesOf(managedModels(cfg)),
		)
	})

	t.Run("offers every built-in model when every family has a key", func(t *testing.T) {
		t.Parallel()

		cfg := config.AIConfig{Managed: config.AIManagedConfig{Anthropic: held, OpenAI: held, OpenWeight: hosts}}

		assert.Len(t, managedModels(cfg), len(aimodels.Managed()))
	})
}
