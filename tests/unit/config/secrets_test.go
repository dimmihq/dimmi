package config_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	frameworkconfig "github.com/dimmihq/dimmi/framework/config"

	"github.com/dimmihq/dimmi/config"
)

/*
publicSetting reports whether a setting named like a secret is safe to
print.
*/
func publicSetting(path string) bool {
	return path == "auth.captcha.site_key"
}

/*
secretNamed reports whether a setting's name ends in a word that names
a secret.
*/
func secretNamed(key string) bool {
	for _, word := range []string{"secret", "password", "token", "key", "dsn", "credentials"} {
		if key == word || strings.HasSuffix(key, "_"+word) {
			return true
		}
	}

	return false
}

/*
secretPaths collects the paths of every string setting in the tree
named like a secret.
*/
func secretPaths(tree map[string]any, path string, found *[]string) {
	for key, value := range tree {
		at := strings.TrimPrefix(path+"."+key, ".")

		switch v := value.(type) {
		case map[string]any:
			secretPaths(v, at, found)
		case string:
			if secretNamed(key) && !publicSetting(at) {
				*found = append(*found, at)
			}
		}
	}
}

/*
settings is a runtime store that supplies fixed values.
*/
type settings map[string]any

func (s settings) Settings(context.Context) (map[string]any, error) {
	return s, nil
}

func lookup(tree map[string]any, path string) any {
	var value any = tree

	for part := range strings.SplitSeq(path, ".") {
		branch, ok := value.(map[string]any)
		if !ok {
			return nil
		}

		value = branch[part]
	}

	return value
}

func TestSecretsAreMaskedWhenPrinted(t *testing.T) {
	t.Parallel()

	var found []string

	secretPaths(frameworkconfig.Redacted(&config.Config{}), "", &found)

	assert.Contains(t, found, "services.slack.signing_secret")
	assert.Contains(t, found, "services.linear.webhook_secret")
	assert.Contains(t, found, "services.trackers.github_webhook_secret")
	assert.Contains(t, found, "ai.managed.anthropic.api_key")
	assert.Contains(t, found, "ai.managed.embeddings.api_key")
	assert.Contains(t, found, "ai.knowledge.cloudflare_api_token")

	values := settings{}
	for _, path := range found {
		values[path] = "value-to-hide"
	}

	var cfg config.Config

	require.NoError(t, frameworkconfig.LoadInto(t.Context(), frameworkconfig.LoadOptions{Runtime: values}, &cfg))

	printed := frameworkconfig.Redacted(&cfg)

	for _, path := range found {
		assert.Equal(t, "[redacted]", lookup(printed, path), path)
	}
}
