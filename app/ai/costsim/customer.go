/*
|--------------------------------------------------------------------------
| Simulated Customers
|--------------------------------------------------------------------------
|
| The customer's side of a simulated conversation. The scripted customer
| sends the scenario's messages in order, whatever the agent says, and
| costs nothing. The model-played customer opens with the scenario's
| first message, then a cheap model writes each next message from the
| persona, the goal and the conversation so far, and says it is done
| when the goal is met or it gives up. Its calls pass the same spend
| guard and are reported apart from the agent's.
|
*/

package costsim

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
DoneMarker is what the model-played customer writes when it is done.
*/
const DoneMarker = "[DONE]"

const (
	metaRole     = "costsim_role"
	roleCustomer = "customer"

	// Offline, the simulation customer's tool call ends the chat at once.
	simulatedCustomerTool = "reply_as_customer"
)

/*
customerPrompt tells the customer's model who it plays and when to stop.
*/
const customerPrompt = `You are role-playing a customer writing to the support chat of %s.
Persona: %s
Your goal: %s
Write only your next chat message to the support agent, as this customer would type it:
short, natural, in the language with code %q. Never say you are an AI or a simulation.
When your goal is met, or you have clearly given up or been handed to a person,
reply with exactly %s and nothing else.`

/*
Turn is one message of a simulated conversation: who wrote it, customer
or agent, and what.
*/
type Turn struct {
	Who  string
	Text string
}

/*
Customer writes the customer's next message, or says the customer is
done. Turn is how many messages the customer has sent so far.
*/
type Customer interface {
	/*
		Next returns the customer's next message, or true when the
		customer is done.
	*/
	Next(ctx context.Context, sc Scenario, turn int, transcript []Turn) (string, bool, error)
}

/*
Scripted sends the scenario's messages in order.
*/
type Scripted struct{}

/*
Next returns the scenario's next message.
*/
func (Scripted) Next(_ context.Context, sc Scenario, turn int, _ []Turn) (string, bool, error) {
	if turn >= len(sc.Messages) {
		return "", true, nil
	}
	return sc.Messages[turn], false, nil
}

/*
Played is the model-played customer.
*/
type Played struct {
	LLM     contracts.LLM
	Model   string
	Company string
}

/*
Next opens with the scenario's first message and has the model write
the rest.
*/
func (p Played) Next(ctx context.Context, sc Scenario, turn int, transcript []Turn) (string, bool, error) {
	if turn == 0 && len(sc.Messages) > 0 {
		return sc.Messages[0], false, nil
	}
	system := fmt.Sprintf(customerPrompt, p.Company, sc.Persona, sc.Goal, sc.Locale, DoneMarker)

	var msgs []contracts.ChatMessage
	for _, t := range transcript {
		role := contracts.ChatRoleUser
		if t.Who == "customer" {
			role = contracts.ChatRoleAssistant
		}
		msgs = append(msgs, contracts.ChatMessage{Role: role, Parts: []contracts.ContentPart{{Text: t.Text}}})
	}
	if !agentSpokeLast(msgs) {
		msgs = append(msgs, contracts.ChatMessage{
			Role:  contracts.ChatRoleUser,
			Parts: []contracts.ContentPart{{Text: "(The agent has not replied. Write your next message.)"}},
		})
	}
	temp := 0.7
	res, err := p.LLM.Chat(ctx, contracts.ChatRequest{
		Model: p.Model, System: []contracts.SystemBlock{{Text: system}}, Messages: msgs, MaxOutputTokens: 300,
		Temperature: &temp, Metadata: map[string]string{metaRole: roleCustomer},
	})
	if err != nil {
		return "", false, fmt.Errorf("customer model: %w", err)
	}
	text := strings.TrimSpace(res.Text)
	if text == "" || strings.Contains(text, DoneMarker) {
		return "", true, nil
	}
	return text, false, nil
}

/*
agentSpokeLast reports whether the last message is the agent's, which
the customer's model reads as the user's turn.
*/
func agentSpokeLast(msgs []contracts.ChatMessage) bool {
	if len(msgs) == 0 {
		return false
	}
	switch msgs[len(msgs)-1].Role {
	case contracts.ChatRoleUser:
		return true
	case contracts.ChatRoleAssistant, contracts.ChatRoleTool:
		return false
	}
	return false
}
