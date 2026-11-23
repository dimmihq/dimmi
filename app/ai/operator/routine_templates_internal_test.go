package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnwrapPrompt(t *testing.T) {
	t.Parallel()

	got := unwrapPrompt("Audit for\n{{workspace_name}}.\n\n1. Call x\n   then y.\n   - one\n     more\n2. Done.")

	assert.Equal(t, "Audit for {{workspace_name}}.\n\n1. Call x then y.\n   - one more\n2. Done.", got)
}

func TestSpellTools(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Call list_articles, then mark_noop.", spellTools("Call [[tool:list_articles]], then [[tool:mark_noop]]."))
}
