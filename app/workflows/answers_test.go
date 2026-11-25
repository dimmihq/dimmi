package workflows

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariablePatternReadsBothFallbackSpellings(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		`{{ contact.first_name | default: "there" }}`,
		`{{contact.first_name | fallback: "there"}}`,
	} {
		m := variablePattern().FindStringSubmatch(text)
		if assert.Len(t, m, 3, text) {
			assert.Equal(t, "contact.first_name", m[1], text)
			assert.Equal(t, "there", m[2], text)
		}
	}

	assert.Equal(t, []string{"{{ contact.name }}", "contact.name", ""},
		variablePattern().FindStringSubmatch("Hi {{ contact.name }}"))
}
