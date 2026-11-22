package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

func TestReadMove(t *testing.T) {
	t.Parallel()

	call := func(args string) *contracts.ChatResponse {
		return &contracts.ChatResponse{ToolCalls: []contracts.ToolCall{
			{Name: customerTool, Arguments: json.RawMessage(args)},
		}}
	}

	move, err := readMove(call(`{"message":"  It's shp_123  ","confirm":false,"done":false,"unknown":false}`), "")
	require.NoError(t, err)
	assert.Equal(t, "It's shp_123", move.Message)

	move, err = readMove(call(`{"message":"","confirm":true,"done":false,"unknown":false}`), "Void shp_1")
	require.NoError(t, err)
	assert.True(t, move.Confirm)

	_, err = readMove(call(`{"message":"","confirm":true,"done":false,"unknown":false}`), "")
	require.ErrorIs(t, err, ErrCustomerFailed, "Confirm cannot be pressed when no card is waiting")

	for _, bad := range []*contracts.ChatResponse{
		{Text: "It's shp_123"},
		call(`{"message":"","confirm":false,"done":false,"unknown":false}`),
		call(`not json`),
	} {
		_, err := readMove(bad, "Void shp_1")
		assert.ErrorIs(t, err, ErrCustomerFailed, "an answer that says nothing usable is refused")
	}
}

func TestCustomerBrief(t *testing.T) {
	t.Parallel()

	brief := customerBrief(CustomerTurn{
		Name:       "Emma",
		Saved:      []SavedMessage{{Text: "Void shp_1"}, {Confirm: true}, {Text: "Keep the balance"}},
		Run:        []SimMessage{{From: "customer", Text: "Void shp_1"}, {From: "agent", Text: "Press Confirm."}},
		Confirming: "Void a label: shp_1",
	})

	assert.Contains(t, brief, "<saved_chat>\nCUSTOMER: Void shp_1\nCUSTOMER pressed Confirm on a card.\n")
	assert.Contains(t, brief, "<this_chat>\nCUSTOMER: Void shp_1\nAGENT: Press Confirm.\n</this_chat>")
	assert.Contains(t, brief, "<card_waiting>Void a label: shp_1</card_waiting>")
}

func TestCustomerSpec(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, string(customerSpec("").Schema), `"confirm"`, "no card, no button to press")
	assert.Contains(t, string(customerSpec("Void shp_1").Schema), `"confirm"`)
}

func TestPersonaContact(t *testing.T) {
	t.Parallel()

	signedIn := personaContact(ids.New(), SimPersona{Name: "Ana", Verified: true})
	require.NotNil(t, signedIn.ExternalID, "a signed-in customer given no id is still known by one")
	again := personaContact(ids.New(), SimPersona{Name: "Ana", Verified: true})
	assert.Equal(t, *signedIn.ExternalID, *again.ExternalID, "and it is the same on every run")

	byEmail := personaContact(ids.New(), SimPersona{Name: "Ana", Email: "ana@example.test", Verified: true})
	assert.Nil(t, byEmail.ExternalID, "an email is enough to know them by")
	assert.True(t, byEmail.EmailVerified, "and a signed-in customer's email is trusted")

	visitor := personaContact(ids.New(), SimPersona{Name: "Ana", Email: "ana@example.test"})
	assert.Nil(t, visitor.ExternalID)
	assert.False(t, visitor.EmailVerified, "a visitor's email is not trusted")
}
