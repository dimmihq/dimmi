package email_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ticketSetup reads the id of the ticket type and of the state named.
*/
func ticketSetup(t *testing.T, owner *testkit.Client, typeName, stateName string) (string, string) {
	t.Helper()

	var types, states struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	owner.GetJSON("/api/v1/ticket-types").AssertOK().Decode(&types)
	owner.GetJSON("/api/v1/ticket-states").AssertOK().Decode(&states)

	pick := func(list []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}, name string,
	) string {
		for _, item := range list {
			if item.Name == name {
				return item.ID
			}
		}

		t.Fatalf("no %s", name)

		return ""
	}

	return pick(types.Data, typeName), pick(states.Data, stateName)
}

/*
withLabel returns the first mail whose text says the label.
*/
func withLabel(t *testing.T, mails [][]byte, label string) []byte {
	t.Helper()

	for _, raw := range mails {
		if strings.Contains(parsed(t, raw).Text, label) {
			return raw
		}
	}

	return nil
}

func TestTicketNoticesByEmail(t *testing.T) {
	t.Parallel()

	t.Run("tells an email ticket's customer of a new state in the same email thread", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("ticket")
		conversation := startConversation(tc, address, customer)
		first := messages(tc, conversation.ID)[0]
		owner := tc.ActingAsOwner().WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace,
			tc.Workspace.Workspace.ID))
		question, investigating := ticketSetup(t, owner, "Question", "Investigating")

		id := owner.PostJSON("/api/v1/conversations/"+models.PublicID(models.PrefixConversation, conversation.ID)+
			"/ticket", testkit.JSON{"type_id": question}).AssertCreated().String("data.id")
		owner.PatchJSON("/api/v1/tickets/"+id, testkit.JSON{"state_id": investigating}).AssertOK()
		tc.Clock.Travel(3 * time.Minute)
		tc.DrainLater()

		raw := withLabel(t, received(t, customer), "We're on it")
		require.NotNil(t, raw, "no state notice mailed")
		got := parsed(t, raw)
		assert.Equal(t, []string{*first.ChannelMessageID}, got.InReplyTo)
		assert.Contains(t, got.References, *first.ChannelMessageID)
		assert.Equal(t, "Re: Refund for order 1042", got.Subject)
		assert.Len(t, conversations(tc), 1)
	})
}
