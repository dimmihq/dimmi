package audiences_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

func TestUntrustedAudienceMessages(t *testing.T) {
	t.Parallel()

	row := models.Audience{ID: ids.New(), Name: "Gmail", Rules: json.RawMessage(
		`{"rules":{"field":"contact.email_domain","op":"eq","value":"gmail.com"}}`,
	)}
	id := models.PublicID(models.PrefixAudience, row.ID)
	resolver := audiences.NewResolver([]models.Audience{row})

	t.Run("says who may read for an article or collection audience", func(t *testing.T) {
		t.Parallel()

		found := resolver.CheckTrusted(id)
		require.Len(t, found, 1)
		assert.Equal(t, audience.CodeClaimedOnAccessGate, found[0].Code)
		assert.Contains(t, found[0].Message, "so it cannot decide who may read.")
	})

	t.Run("says who may start a conversation for a messenger's start rule", func(t *testing.T) {
		t.Parallel()

		start, err := audience.Parse(json.RawMessage(`{"rules":{"field":"audience","op":"in","value":["` + id + `"]}}`))
		require.NoError(t, err)

		found := resolver.CheckNamed(audiences.MessengerRules{Start: &start})
		require.Len(t, found, 1)
		assert.Contains(t, found[0].Message, "so it cannot decide who may start a conversation.")
	})
}
