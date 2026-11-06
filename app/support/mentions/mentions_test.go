package mentions_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/mentions"
)

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads members and teams once each in the order written", func(t *testing.T) {
		t.Parallel()

		got := mentions.Parse("Hey <@mem_abc> and <@team_Billing1>, also <@mem_abc> again")

		assert.Equal(t, []mentions.Token{
			{Kind: mentions.KindMember, ID: "mem_abc"}, {Kind: mentions.KindTeam, ID: "team_Billing1"},
		}, got)
	})

	t.Run("leaves addresses, other ids and broken tokens as text", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, mentions.Parse("mail <ada@example.com> <@ctc_abc> <@mem_> <@mem_a-b> @mem_abc"))
	})

	t.Run("stops at the most a note may mention", func(t *testing.T) {
		t.Parallel()

		var b strings.Builder
		for i := range 80 {
			b.WriteString("<@mem_a" + strconv.Itoa(i) + "> ")
		}

		assert.Len(t, mentions.Parse(b.String()), mentions.MaxTokens)
	})
}

func FuzzParse(f *testing.F) {
	f.Add("<@mem_abc> <@team_x>")
	f.Add("<@<@mem_a>>")

	f.Fuzz(func(t *testing.T, body string) {
		got := mentions.Parse(body)

		assert.LessOrEqual(t, len(got), mentions.MaxTokens)

		for _, token := range got {
			assert.Contains(t, body, "<@"+token.ID+">")
		}
	})
}
