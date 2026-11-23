package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
voidChat is a saved chat of three moves, the first sent, with dimmi
waiting on a card the customer may confirm.
*/
func voidChat() CustomerTurn {
	return CustomerTurn{
		Name: "Emma", Scenario: "Emma wants the label shp_1 voided.", confirmable: true,
		Saved: []SavedMessage{{Text: "Void shp_1", Sent: true}, {Confirm: true}, {Text: "Keep the balance"}},
		Run: []SimMessage{
			{From: "customer", Text: "Void shp_1"}, {From: "agent", Text: "Press Confirm to void it."},
		},
		Confirming: "Void a label: shp_1",
	}
}

func TestReadMove(t *testing.T) {
	t.Parallel()

	call := func(args string) *contracts.ChatResponse {
		return &contracts.ChatResponse{ToolCalls: []contracts.ToolCall{
			{Name: customerTool, Arguments: json.RawMessage(args)},
		}}
	}
	in := voidChat()

	move, err := readMove(call(`{"move":"answer","saved":0,"message":"  It's shp_123  ","unknown":true}`), in)
	require.NoError(t, err)
	assert.Equal(t, CustomerMove{Message: "It's shp_123", Unknown: true}, move)

	move, err = readMove(call(`{"move":"send_saved","saved":3,"message":"","unknown":false}`), in)
	require.NoError(t, err)
	assert.Equal(t, CustomerMove{Message: "Keep the balance", Saved: 3}, move, "a saved message goes out word for word")

	move, err = readMove(call(`{"move":"confirm","saved":0,"message":"","unknown":false}`), in)
	require.NoError(t, err)
	assert.Equal(t, CustomerMove{Confirm: true, Saved: 2}, move, "the press is the saved chat's own")

	move, err = readMove(&contracts.ChatResponse{Text: " No thanks, that's all. "}, in)
	require.NoError(t, err)
	assert.Equal(t, CustomerMove{Message: "No thanks, that's all."}, move, "words without the tool are the answer")

	move, err = readMove(call(`{"move":"end","saved":0,"message":"","unknown":false}`), in)
	require.NoError(t, err)
	assert.True(t, move.Done)

	notTheirs := in
	notTheirs.confirmable = false
	_, err = readMove(call(`{"move":"confirm","saved":0,"message":"","unknown":false}`), notTheirs)
	require.ErrorIs(t, err, ErrCustomerFailed, "a card for something the customer did not come for cannot be confirmed")

	noCard := in
	noCard.Confirming, noCard.confirmable = "", false
	for name, bad := range map[string]*contracts.ChatResponse{
		"a message already sent":       call(`{"move":"send_saved","saved":1,"message":"","unknown":false}`),
		"a saved press sent as a text": call(`{"move":"send_saved","saved":2,"message":"","unknown":false}`),
		"an answer that says nothing":  call(`{"move":"answer","saved":0,"message":"  ","unknown":false}`),
		"a move that does not exist":   call(`{"move":"repeat","saved":0,"message":"hi","unknown":false}`),
		"no tool call and no words":    {Text: "  "},
		"arguments that do not parse":  call(`not json`),
	} {
		_, err := readMove(bad, in)
		assert.ErrorIs(t, err, ErrCustomerFailed, name)
	}
	_, err = readMove(call(`{"move":"confirm","saved":0,"message":"","unknown":false}`), noCard)
	require.ErrorIs(t, err, ErrCustomerFailed, "Confirm cannot be pressed when no card is waiting")
}

func TestScriptedMove(t *testing.T) {
	t.Parallel()

	saved := []SavedMessage{{Text: "Void shp_1", Sent: true}, {Confirm: true}, {Text: "Keep the balance"}}
	assert.Equal(t, CustomerMove{Message: "Keep the balance", Saved: 3}, scripted(saved),
		"the next unsent message goes out, passing over a press with no card to press")

	saved[2].Sent = true
	assert.Equal(t, CustomerMove{Done: true}, scripted(saved), "with nothing left to say the chat ends")
}

func TestCustomerChat(t *testing.T) {
	t.Parallel()

	chat := customerChat(voidChat())

	require.Len(t, chat, 3)
	assert.Equal(t, contracts.ChatRoleUser, chat[0].Role)
	assert.Contains(t, chat[0].Parts[0].Text, "<scenario>Emma wants the label shp_1 voided.</scenario>")
	assert.Contains(t, chat[0].Parts[0].Text, "<customer>Emma</customer>")
	assert.Contains(t, chat[0].Parts[0].Text,
		"<saved_messages>\n1. Void shp_1\n2. (pressed Confirm on the card)\n3. Keep the balance\n</saved_messages>")
	assert.Equal(t, contracts.ChatRoleAssistant, chat[1].Role, "the customer's own message is its own turn")
	assert.Equal(t, "Void shp_1", chat[1].Parts[0].Text)
	assert.Equal(t, contracts.ChatRoleUser, chat[2].Role)
	assert.Contains(t, chat[2].Parts[0].Text, "<agent>Press Confirm to void it.</agent>")
	assert.Contains(t, chat[2].Parts[0].Text, "Saved messages not sent yet: 3.")
	assert.Contains(t, chat[2].Parts[0].Text, "A card is waiting for the customer to confirm: Void a label: shp_1")

	pressed := voidChat()
	pressed.Confirming, pressed.confirmable = "", false
	pressed.Run = append(pressed.Run, SimMessage{From: "agent", Text: "Voided. Refund or balance?"})
	chat = customerChat(pressed)
	require.Len(t, chat, 5)
	assert.Equal(t, "(pressed Confirm on the card)", chat[3].Parts[0].Text, "a press shows as the customer's turn")
	assert.Contains(t, chat[4].Parts[0].Text, "No card is waiting.")
}

func TestCustomerSpec(t *testing.T) {
	t.Parallel()

	in := voidChat()
	schema := string(customerSpec(in).Schema)
	assert.Contains(t, schema, `"enum":["answer","send_saved","confirm","end"]`)
	assert.Contains(t, schema, `"enum":[0,3]`, "only the unsent message is on offer")

	in.confirmable = false
	assert.NotContains(t, string(customerSpec(in).Schema), `"confirm"`,
		"a card for something the customer did not come for offers no button")

	in.Confirming = ""
	in.Saved[2].Sent = true
	schema = string(customerSpec(in).Schema)
	assert.Contains(t, schema, `"enum":["answer","end"]`, "no card and nothing unsent leave only an answer or the end")
	assert.Contains(t, schema, `"enum":[0]`)
}

func TestCustomerPrompt(t *testing.T) {
	t.Parallel()

	require.NoError(t, prompts.CheckTasks())
	system := prompts.Task("simulation/customer", nil)
	assert.Contains(t, system, customerTool)
	assert.NotContains(t, system, "<!--", "the editors' note is never sent")
	assert.NotEmpty(t, prompts.Task("simulation/scenario", nil))
	for _, name := range []string{"reply_asks", "simulation_check", "customer_wants"} {
		q := prompts.Judgment(name, prompts.Vars{"reply": "R", "transcript": "T", "check": "C", "card": "K"})
		assert.NotEmpty(t, q.State, name)
		assert.NotEmpty(t, q.Ask, name)
		assert.NotEmpty(t, q.True, name)
		assert.NotEmpty(t, q.False, name)
	}
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
