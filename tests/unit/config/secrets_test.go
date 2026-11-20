package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	frameworkconfig "github.com/dimmihq/dimmi/framework/config"

	"github.com/dimmihq/dimmi/config"
)

/*
public lists the settings named like a secret that are safe to print.
*/
var public = map[string]bool{
	"auth.captcha.site_key": true,
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
fill sets every string setting named like a secret and returns their
paths.
*/
func fill(v reflect.Value, path string, found *[]string) {
	for i := range v.NumField() {
		field := v.Type().Field(i)

		key, _, _ := strings.Cut(field.Tag.Get("koanf"), ",")
		if !field.IsExported() || key == "" || key == "-" {
			continue
		}

		at := strings.TrimPrefix(path+"."+key, ".")

		if field.Type.Kind() == reflect.Struct {
			fill(v.Field(i), at, found)

			continue
		}

		if field.Type.Kind() == reflect.String && secretNamed(key) && !public[at] {
			v.Field(i).SetString("value-to-hide")
			*found = append(*found, at)
		}
	}
}

func lookup(tree map[string]any, path string) any {
	var value any = tree

	for part := range strings.SplitSeq(path, ".") {
		value = value.(map[string]any)[part]
	}

	return value
}

func TestSecretsAreMaskedWhenPrinted(t *testing.T) {
	t.Parallel()

	var (
		cfg   config.Config
		found []string
	)

	fill(reflect.ValueOf(&cfg).Elem(), "", &found)

	assert.Contains(t, found, "services.slack.signing_secret")
	assert.Contains(t, found, "services.linear.webhook_secret")
	assert.Contains(t, found, "services.trackers.github_webhook_secret")
	assert.Contains(t, found, "ai.managed.anthropic.api_key")
	assert.Contains(t, found, "ai.managed.embeddings.api_key")
	assert.Contains(t, found, "ai.knowledge.cloudflare_api_token")

	printed := frameworkconfig.Redacted(&cfg)

	for _, path := range found {
		assert.Equal(t, "[redacted]", lookup(printed, path), path)
	}
}
