package channels_test

import (
	"encoding/json"
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	slackevents "github.com/dimmihq/dimmi/tests/fixtures/channels/slack/events"
)

/*
recorded reads a recorded Slack delivery.
*/
func recorded(t *testing.T, name string) map[string]any {
	t.Helper()

	raw, err := fs.ReadFile(slackevents.Files, name+".json")
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))

	return out
}

/*
slackTicket is a ticket as the intake made it.
*/
type slackTicket struct {
	ID             ids.UUID
	Number         int64
	TypeID         ids.UUID
	ConversationID ids.UUID
	CreatedBy      string
	Title          string
}

/*
tickets returns the workspace's tickets, oldest first.
*/
func (sc *slackCase) tickets() []slackTicket {
	sc.T.Helper()

	rows, err := sc.Operator().Query(sc.T.Context(),
		"SELECT id, number, type_id, conversation_id, created_by_type, title FROM tickets "+
			"WHERE workspace_id = $1 ORDER BY number", sc.Workspace.Workspace.ID)
	require.NoError(sc.T, err)

	defer rows.Close()

	var out []slackTicket

	for rows.Next() {
		var tk slackTicket
		require.NoError(sc.T, rows.Scan(&tk.ID, &tk.Number, &tk.TypeID, &tk.ConversationID, &tk.CreatedBy, &tk.Title))
		out = append(out, tk)
	}

	return out
}

/*
ticketType returns the public id of the workspace's ticket type named.
*/
func (sc *slackCase) ticketType(name string) string {
	sc.T.Helper()

	res := sc.In(sc.Owner).GetJSON("/api/v1/ticket-types").AssertOK()

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, ty := range body.Data {
		if ty.Name == name {
			return ty.ID
		}
	}

	sc.T.Fatalf("no ticket type %s", name)

	return ""
}

func TestSlackTicketIntake(t *testing.T) {
	t.Parallel()

	t.Run("a teammate's ticket emoji makes the thread's conversation one ticket", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")

		sc.deliver(recorded(t, "message.customer"))
		convs := sc.conversations()
		require.Len(t, convs, 1)

		sc.deliver(recorded(t, "reaction_added.ticket"))
		sc.deliver(recorded(t, "reaction_added.ticket"))

		made := sc.tickets()
		require.Len(t, made, 1)
		assert.Equal(t, convs[0].ID, made[0].ConversationID)
		assert.Equal(t, "member", made[0].CreatedBy)
		assert.NotEmpty(t, made[0].Title)
	})

	t.Run("in an emoji room the ticket emoji opens the conversation and makes the ticket", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "emoji")

		sc.deliver(recorded(t, "message.customer"))
		assert.Empty(t, sc.conversations())

		sc.deliver(recorded(t, "reaction_added.ticket"))

		convs := sc.conversations()
		require.Len(t, convs, 1)
		made := sc.tickets()
		require.Len(t, made, 1)
		assert.Equal(t, convs[0].ID, made[0].ConversationID)
		assert.Equal(t, "The export *keeps* failing", made[0].Title)
	})

	t.Run("a customer's ticket emoji and other emoji make nothing", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")
		sc.deliver(recorded(t, "message.customer"))

		byCustomer := recorded(t, "reaction_added.ticket")
		byCustomer["user"] = "U0CUSTOMER"
		sc.deliver(byCustomer)

		other := recorded(t, "reaction_added.ticket")
		other["reaction"] = "eyes"
		sc.deliver(other)

		assert.Len(t, sc.conversations(), 1)
		assert.Empty(t, sc.tickets())
	})

	t.Run("the room's ticket type is used", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")

		bug := sc.ticketType("Bug report")
		_, err := sc.Operator().Exec(sc.T.Context(),
			"UPDATE channel_rooms SET settings = settings || jsonb_build_object('ticket_type_id', $1::text) "+
				"WHERE channel_connection_id = $2 AND external_id = $3", bug, sc.conn.ID, supportRoom)
		require.NoError(t, err)

		sc.deliver(recorded(t, "message.customer"))
		sc.deliver(recorded(t, "reaction_added.ticket"))

		made := sc.tickets()
		require.Len(t, made, 1)
		assert.Equal(t, bug, models.PublicID(models.PrefixTicketType, made[0].TypeID))
	})
}

func TestSlackTicketShortcut(t *testing.T) {
	t.Parallel()

	t.Run("the shortcut makes a ticket once and answers with its reference", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "off")

		sc.interact(recorded(t, "message_action.ticket"))
		again := recorded(t, "message_action.ticket")
		again["trigger_id"] = "trig-2"
		sc.interact(again)

		require.Len(t, sc.conversations(), 1)
		made := sc.tickets()
		require.Len(t, made, 1)

		ephemerals := sc.slack.called("chat.postEphemeral")
		require.Len(t, ephemerals, 2)
		assert.Contains(t, ephemerals[0].Form.Get("text"), "Made ticket T-")
		assert.Contains(t, ephemerals[1].Form.Get("text"), "Already ticket T-")
	})

	t.Run("the shortcut on a teammate's message explains why not", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "off")

		own := recorded(t, "message_action.ticket")
		msg, ok := own["message"].(map[string]any)
		require.True(t, ok)
		msg["user"] = "U0TEAMMATE"
		sc.interact(own)

		assert.Empty(t, sc.tickets())
		ephemerals := sc.slack.called("chat.postEphemeral")
		require.Len(t, ephemerals, 1)
		assert.Contains(t, ephemerals[0].Form.Get("text"), "customer's thread")
	})

	t.Run("a state change on the ticket is posted in the Slack thread", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")
		sc.deliver(recorded(t, "message.customer"))
		sc.deliver(recorded(t, "reaction_added.ticket"))

		made := sc.tickets()
		require.Len(t, made, 1)

		before := len(sc.slack.called("chat.postMessage"))
		sc.In(sc.Owner).PatchJSON("/api/v1/tickets/"+models.PublicID(models.PrefixTicket, made[0].ID),
			testkit.JSON{"state_id": sc.ticketState("Investigating")}).AssertOK()
		sc.Clock.Travel(3 * time.Minute)
		sc.DrainLater()

		posts := sc.slack.called("chat.postMessage")
		require.Greater(t, len(posts), before)
		assert.Contains(t, posts[len(posts)-1].Body+posts[len(posts)-1].Form.Encode(), "1730000300.000100")
	})
}

/*
ticketState returns the public id of the workspace's ticket state named.
*/
func (sc *slackCase) ticketState(name string) string {
	sc.T.Helper()

	res := sc.In(sc.Owner).GetJSON("/api/v1/ticket-states").AssertOK()

	var body struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	res.Decode(&body)

	for _, st := range body.Data {
		if st.Name == name {
			return st.ID
		}
	}

	sc.T.Fatalf("no ticket state %s", name)

	return ""
}
