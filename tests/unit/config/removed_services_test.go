package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	frameworkconfig "github.com/dimmihq/dimmi/framework/config"

	"github.com/dimmihq/dimmi/config"
)

/*
An older install may still set the Intercom and Zendesk OAuth clients
dimmi no longer connects through, in its dimmi.yaml or its environment;
it still boots and ignores them.
*/
func TestConfigurationIgnoresRemovedServices(t *testing.T) {
	t.Parallel()

	var cfg config.Config

	err := frameworkconfig.LoadInto(t.Context(), frameworkconfig.LoadOptions{
		EnvPrefix: "DIMMI",
		Environ: []string{
			"DIMMI_SERVICES__INTERCOM__CLIENT_ID=old-intercom",
			"DIMMI_SERVICES__INTERCOM__CLIENT_SECRET=old-secret",
		},
		Runtime: settings{"services": map[string]any{
			"zendesk": map[string]any{"client_id": "old-zendesk", "client_secret": "old-secret"},
			"notion":  map[string]any{"client_id": "notion-client"},
		}},
	}, &cfg)
	require.NoError(t, err)
	assert.Equal(t, "notion-client", cfg.Services.Notion.ClientID)
	assert.NotContains(t, cfg.Integrations.Enabled, "intercom")
	assert.NotContains(t, cfg.Integrations.Enabled, "zendesk")
}
