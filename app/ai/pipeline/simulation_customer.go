/*
|--------------------------------------------------------------------------
| The Simulated Customer
|--------------------------------------------------------------------------
|
| A saved simulation keeps the chat it was made from. When it runs,
| that customer is played again, and what they do next follows from
| where the chat stands, not from a model's reading of it:
|
|   - The run keeps which saved messages have been sent. A sent message
|     is never offered again, so the customer cannot repeat itself.
|   - When the agent's reply asks nothing of the customer, the next
|     saved message goes out word for word, and when none is left the
|     chat ends. No model is asked.
|   - When the agent's reply needs something back, a model answers as
|     the customer. It sits in the chat as the customer, its own
|     earlier messages as its own turns, and picks one move from those
|     the chat allows now: send a saved message that answers, write an
|     answer from what the customer knows, press Confirm on a waiting
|     card, or end the chat. Confirm is on offer only for something
|     this customer came for: a press the saved chat holds, or an
|     action the scenario and saved messages show them asking for.
|
| A message the model writes in its own words is then held to what is
| known about the customer, in simulation_customer_grounded.go.
|
| Whether a reply asks for something is the decision model's call.
| Where no decision model is set up, or it cannot answer, the customer
| model is asked every turn instead and decides for itself.
|
| The customer is played on dimmi's built-in provider by Claude Haiku
| 5.5, or by GPT-6 Luna on an install with no Anthropic upstream: a
| model that keeps to a role and to the tool it is given. Its prompt is
| resources/prompts/tasks/simulation/customer.md. Its move comes back
| through a single tool whose arguments are checked in code; a call
| that fails, a reply with no tool call, or a move the chat does not
| allow is asked once more, and after that the run fails and can simply
| be run again. Every call is recorded on a simulation run and spends
| like any other.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
customerModel is the model that plays the customer on this install, on
the built-in provider: Claude Haiku 5.5, GPT-6 Luna on an install with
no Anthropic upstream, and empty when it has neither. Both keep to a
role and answer through the tool they are given, which playing a
customer needs.
*/
func (r *Runner) customerModel() string {
	served := r.LLM.ManagedModels()
	for _, id := range []string{"claude-haiku-5-5", "gpt-6-luna"} {
		if aimodels.Find(served, id) != nil {
			return id
		}
	}

	return ""
}

/*
customerTool is the one tool the simulated customer answers through.
*/
const customerTool = "customer_move"

/*
The moves the simulated customer may make.
*/
const (
	moveSendSaved = "send_saved"
	moveAnswer    = "answer"
	moveConfirm   = "confirm"
	moveEnd       = "end"
	moveTap       = "tap"
	moveWait      = "wait"
)

/*
pressedConfirm stands in the chat for a turn where the customer pressed
Confirm on a card and wrote nothing.
*/
const pressedConfirm = "(pressed Confirm on the card)"

/*
customerTries is how many times the customer is asked before the run
fails; customerTokens bounds its answer.
*/
const (
	customerTries  = 2
	customerTokens = 400
)

/*
ErrCustomerUnavailable is returned when dimmi's built-in provider, which
plays the customer, is not set up for the workspace, or serves neither
of the models that play it.
*/
var ErrCustomerUnavailable = errors.New("the simulated customer needs dimmi's built-in Claude or GPT models")

/*
ErrCustomerFailed is returned when the simulated customer gave no
usable answer after every try.
*/
var ErrCustomerFailed = errors.New("the simulated customer did not answer")

/*
CustomerTurn is where a simulated chat stands when the customer moves
next: who the customer is, the simulation's scenario, the saved chat
with what has been sent of it, the chat of this run so far, and the
card dimmi is waiting on, if any. In a workflow, Choices are the
buttons the customer may tap now, ChoicesOnly says they may not type
instead, and CanWait says the workflow waits for an answer the customer
may simply not give, so that a timeout passes. Whether the customer may press
Confirm on that card is worked out when the turn is played, and so is
an answer of the customer's model that strayed from what is known
about the customer and is being asked again.
*/
type CustomerTurn struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	Name        string
	Scenario    string
	Saved       []SavedMessage
	Run         []SimMessage
	Confirming  string
	Choices     []string
	ChoicesOnly bool
	CanWait     bool
	confirmable bool
	strayed     string
}

/*
SavedMessage is one thing the customer did in the saved chat: wrote a
message, or pressed Confirm on a card. Sent says it has already
happened in this run.
*/
type SavedMessage struct {
	Text    string
	Confirm bool
	Sent    bool
}

/*
CustomerMove is the simulated customer's next move: a message, a press
of Confirm, or the end of the chat. Saved is the number of the saved
message the move sends or the saved press it makes, counting from one,
and zero for a move that is neither. Unknown says the customer was
asked for something it does not know. Choice is the number of the
button the customer tapped, counting from one, and Waited says the
customer let the workflow's wait run out without answering.
*/
type CustomerMove struct {
	Message string
	Confirm bool
	Done    bool
	Unknown bool
	Saved   int
	Choice  int
	Waited  bool
}

/*
PlayCustomer returns the simulated customer's next move.
*/
func (r *Runner) PlayCustomer(ctx context.Context, in CustomerTurn) (CustomerMove, error) {
	r.prepare()
	model := r.customerModel()
	if model == "" {
		return CustomerMove{}, ErrCustomerUnavailable
	}
	st, key, err := r.openCustomer(ctx, in.WorkspaceID, in.MemberID)
	if err != nil {
		return CustomerMove{}, err
	}
	in.confirmable = r.mayConfirm(ctx, st, in)

	if asks, known := r.customerAsked(ctx, st, in); known && !asks {
		return scripted(in.Saved), r.closeCustomer(ctx, st, nil)
	}

	llm, err := r.LLM.ForKey(ctx, key)
	if err != nil {
		return CustomerMove{}, r.closeCustomer(ctx, st, err)
	}
	asked := customerCall{st: st, llm: llm, key: key, model: model}
	var move CustomerMove
	err = ErrCustomerFailed
	for range customerTries {
		if move, err = r.askCustomer(ctx, asked, in); err == nil {
			break
		}
	}
	if err == nil {
		move = r.grounded(ctx, asked, in, move)
	}

	return move, r.closeCustomer(ctx, st, err)
}

/*
scripted is the customer's move when dimmi's reply asked nothing of
them: the next saved message not yet sent, word for word, or the end of
the chat when none is left. A saved press of Confirm is passed over,
since there is no card to press.
*/
func scripted(saved []SavedMessage) CustomerMove {
	for i, m := range saved {
		if !m.Sent && !m.Confirm {
			return CustomerMove{Message: m.Text, Saved: i + 1}
		}
	}

	return CustomerMove{Done: true}
}

/*
customerAsked reports whether dimmi's latest reply needs something back
from the customer, and whether that could be told. A waiting card
always does. With no decision model, or none that answers, it cannot be
told and the customer model decides.
*/
func (r *Runner) customerAsked(ctx context.Context, st *run, in CustomerTurn) (asks, known bool) {
	if in.Confirming != "" || len(in.Choices) > 0 || in.CanWait {
		return true, true
	}
	reply := ""
	if n := len(in.Run); n > 0 && in.Run[n-1].From == "agent" {
		reply = in.Run[n-1].Text
	}
	if strings.TrimSpace(reply) == "" {
		return false, true
	}
	q := prompts.Judgment("reply_asks", prompts.Vars{"reply": truncate(r.redactFor(st.env, reply), maxReplyChars)})
	answers := r.decideTwice(ctx, st, contracts.DecisionRequest{
		State: q.State,
		Questions: map[string]contracts.DecisionQuestion{"asks": {
			Type: contracts.DecisionNoul, Instructions: q.Ask, True: q.True, False: q.False,
		}},
	})
	likely, ok := answers["asks"]

	return likely >= judged, ok
}

/*
openCustomer opens the run the customer's calls are recorded on and
finds the workspace's built-in connection.
*/
func (r *Runner) openCustomer(ctx context.Context, workspace, member ids.UUID,
) (*run, models.AiProviderKey, error) {
	var st *run
	var key models.AiProviderKey
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e, _, err := r.loadSimulation(ctx, tx, SimulationInput{WorkspaceID: workspace})
		if err != nil {
			return err
		}
		key, err = models.New(tx).FindManagedProviderKey(ctx, workspace)
		if errors.Is(err, database.ErrNotFound) {
			return ErrCustomerUnavailable
		}
		if err != nil {
			return err
		}
		args := Args{WorkspaceID: workspace, Kind: simulationKind, MemberID: &member}
		row, err := r.createRun(ctx, tx, e, args)
		if err != nil {
			return err
		}
		st = &run{env: e, rec: r.newRecorder(row), args: args, settings: settingsFor(e)}
		return nil
	})

	return st, key, err
}

/*
mayConfirm reports whether the customer may press Confirm on the card
dimmi is waiting on. A press the saved chat holds and has not used is
theirs to make. Without one, the customer may confirm only what they
came for, which the decision model reads from the scenario and the
saved messages: a simulation never approves an action its customer did
not ask for, and one whose customer did ask is not stopped by a
confirmation step added since it was saved. With no decision model, or
none that answers, a scenario is taken to say what the customer wants
and the customer's model decides from it; with neither, the button is
not offered.
*/
func (r *Runner) mayConfirm(ctx context.Context, st *run, in CustomerTurn) bool {
	if in.Confirming == "" {
		return false
	}
	if savedPress(in.Saved) > 0 {
		return true
	}
	q := prompts.Judgment("customer_wants", prompts.Vars{
		"scenario": cmpOr(strings.TrimSpace(in.Scenario), "(nothing)"),
		"saved":    savedLines(in.Saved),
		"card":     in.Confirming,
	})
	answers := r.decideTwice(ctx, st, contracts.DecisionRequest{
		State: q.State,
		Questions: map[string]contracts.DecisionQuestion{"wants": {
			Type: contracts.DecisionNoul, Instructions: q.Ask, True: q.True, False: q.False,
		}},
	})
	if likely, ok := answers["wants"]; ok {
		return likely >= judged
	}

	return strings.TrimSpace(in.Scenario) != ""
}

/*
savedLines lists the saved chat, numbered, a press of Confirm shown as
one.
*/
func savedLines(saved []SavedMessage) string {
	var b strings.Builder
	for i, m := range saved {
		text := m.Text
		if m.Confirm {
			text = pressedConfirm
		}
		b.WriteString(strconv.Itoa(i+1) + ". " + text + "\n")
	}

	return strings.TrimSpace(b.String())
}

/*
closeCustomer finishes the customer's run, as failed when it gave no
move, and returns the error the caller sees.
*/
func (r *Runner) closeCustomer(ctx context.Context, st *run, err error) error {
	d := &decision{outcome: outcomeAnswered}
	if err != nil {
		d = failed("provider_error", err)
	}
	_ = r.DB.Transaction(context.WithoutCancel(ctx), func(ctx context.Context, tx *database.Tx) error {
		if err := r.finishRun(ctx, tx, st, d); err != nil {
			return err
		}
		return r.recordSpend(ctx, tx, st, d)
	})

	return err
}

/*
customerCall is where a call to the customer's model goes and is kept:
its run, the model, its connection and the key it is billed to.
*/
type customerCall struct {
	st    *run
	llm   contracts.LLM
	key   models.AiProviderKey
	model string
}

/*
askCustomer makes one call to the customer's model and reads its move.
*/
func (r *Runner) askCustomer(ctx context.Context, c customerCall, in CustomerTurn) (CustomerMove, error) {
	st, llm, key := c.st, c.llm, c.key
	req := contracts.ChatRequest{
		Model:      c.model,
		System:     []contracts.SystemBlock{{Text: prompts.Task("simulation/customer", nil), Cacheable: true}},
		Messages:   customerChat(in),
		Tools:      []contracts.ToolSpec{customerSpec(in)},
		ToolChoice: contracts.ToolChoiceAuto, MaxOutputTokens: customerTokens, Effort: contracts.EffortLow,
	}
	ctx, cancel := r.timeout(ctx, r.Timeouts.GenerateTotal)
	defer cancel()
	started := r.Clock.Now()
	res, err := llm.Chat(ctx, req)
	if res == nil {
		res = &contracts.ChatResponse{}
	}
	calls, _ := json.Marshal(res.ToolCalls)
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindGenerate, llm: llm, key: &keyRef{id: key.ID, provider: key.Provider},
		model: c.model, started: started, request: transcriptOf(req), response: res.Text + "\n" + string(calls),
		usage: res.Usage, err: err, requestID: res.RequestID, responseModel: res.Model,
	})
	if err != nil {
		return CustomerMove{}, err
	}

	return readMove(res, in)
}

/*
unsent lists the numbers, counting from one, of the saved messages the
customer has not sent in this run. Saved presses of Confirm are not
messages and are left out.
*/
func unsent(saved []SavedMessage) []int {
	var out []int
	for i, m := range saved {
		if !m.Sent && !m.Confirm {
			out = append(out, i+1)
		}
	}

	return out
}

/*
toldMove is the move as the customer's model told it through the tool.
*/
type toldMove struct {
	Move    string `json:"move"`
	Saved   int    `json:"saved"`
	Message string `json:"message"`
	Unknown bool   `json:"unknown"`
	Choice  int    `json:"choice"`
}

/*
readMove reads the customer's move from its tool call. A call that
cannot be read, or asks for a move the chat does not allow, is refused
and asked again. So is a reply with no tool call: words written outside
the tool are the model thinking aloud as often as they are the
customer's answer, and nothing tells the two apart, so they are never
sent as the customer.
*/
func readMove(res *contracts.ChatResponse, in CustomerTurn) (CustomerMove, error) {
	for _, call := range res.ToolCalls {
		if call.Name != customerTool {
			continue
		}
		var told toldMove
		if json.Unmarshal(call.Arguments, &told) != nil {
			return CustomerMove{}, ErrCustomerFailed
		}
		if move, ok := allowed(told, in); ok {
			return move, nil
		}
		return CustomerMove{}, ErrCustomerFailed
	}

	return CustomerMove{}, ErrCustomerFailed
}

/*
allowed holds a told move to what the chat allows: a saved message must
be one still unsent, an answer must say something, and Confirm needs a
card the customer may confirm.
*/
func allowed(told toldMove, in CustomerTurn) (CustomerMove, bool) {
	switch told.Move {
	case moveSendSaved:
		if in.ChoicesOnly && len(in.Choices) > 0 {
			return CustomerMove{}, false
		}
		if slices.Contains(unsent(in.Saved), told.Saved) {
			return CustomerMove{Message: in.Saved[told.Saved-1].Text, Saved: told.Saved}, true
		}
	case moveAnswer:
		if in.ChoicesOnly && len(in.Choices) > 0 {
			return CustomerMove{}, false
		}
		if text := strings.TrimSpace(told.Message); text != "" {
			return CustomerMove{Message: text, Unknown: told.Unknown}, true
		}
	case moveConfirm:
		if in.confirmable {
			return CustomerMove{Confirm: true, Saved: savedPress(in.Saved)}, true
		}
	case moveEnd:
		return CustomerMove{Done: true}, true
	case moveTap, moveWait:
		return workflowMove(told, in)
	}

	return CustomerMove{}, false
}

/*
savedPress is the number of the first saved press of Confirm not yet
made in this run, or zero when the saved chat has none left.
*/
func savedPress(saved []SavedMessage) int {
	for i, m := range saved {
		if m.Confirm && !m.Sent {
			return i + 1
		}
	}

	return 0
}

/*
customerChat is the chat as the customer's model sits in it. The first
message tells it who it is and lists the saved chat. After that its own
earlier messages are its own turns and each of dimmi's replies arrives
from the other side, in <agent> tags. The last message ends with where
the chat stands: the saved messages still unsent and the card waiting.
*/
func customerChat(in CustomerTurn) []contracts.ChatMessage {
	turns := []chatTurn{{role: contracts.ChatRoleUser, text: customerOpening(in)}}
	add := func(role contracts.ChatRole, text string) {
		if last := &turns[len(turns)-1]; last.role == role {
			last.text += "\n\n" + text
			return
		}
		turns = append(turns, chatTurn{role: role, text: text})
	}
	for i, m := range in.Run {
		if m.From == "customer" {
			add(contracts.ChatRoleAssistant, m.Text)
			continue
		}
		if i > 0 && in.Run[i-1].From == "agent" {
			add(contracts.ChatRoleAssistant, pressedConfirm)
		}
		add(contracts.ChatRoleUser, "<agent>"+escape(m.Text)+"</agent>")
	}
	add(contracts.ChatRoleUser, customerStatus(in))

	out := make([]contracts.ChatMessage, len(turns))
	for i, t := range turns {
		out[i] = contracts.ChatMessage{Role: t.role, Parts: []contracts.ContentPart{{Text: t.text}}}
	}

	return out
}

/*
chatTurn is one side's turn in the customer's chat.
*/
type chatTurn struct {
	role contracts.ChatRole
	text string
}

/*
customerOpening is the first thing the customer's model reads: the
scenario, who the customer is and the saved chat, numbered.
*/
func customerOpening(in CustomerTurn) string {
	scenario := ""
	if text := strings.TrimSpace(in.Scenario); text != "" {
		scenario = "<scenario>" + escape(text) + "</scenario>\n"
	}

	return scenario + savedChat(in.Name, in.Saved) + "\nThe chat starts now. You write first, as the customer."
}

/*
savedChat is who the customer is and what they did in the saved chat,
numbered, as a model reads it.
*/
func savedChat(name string, saved []SavedMessage) string {
	var b strings.Builder
	if name != "" {
		b.WriteString("<customer>" + escape(name) + "</customer>\n")
	}
	b.WriteString("<saved_messages>\n" + escape(savedLines(saved)) + "\n</saved_messages>")

	return b.String()
}

/*
customerStatus is where the chat stands for the customer's next move.
*/
func customerStatus(in CustomerTurn) string {
	left := unsent(in.Saved)
	status := "Every saved message has been sent."
	if len(left) > 0 {
		numbers := make([]string, len(left))
		for i, number := range left {
			numbers[i] = strconv.Itoa(number)
		}
		status = "Saved messages not sent yet: " + strings.Join(numbers, ", ") + "."
	}
	if in.Confirming != "" {
		status += "\nA card is waiting for the customer to confirm: " + escape(in.Confirming)
	} else {
		status += "\nNo card is waiting."
	}
	status += workflowStatus(in)

	note := ""
	if in.strayed != "" {
		note = strayedNote(in.strayed)
	}

	return note + "<status>\n" + status + "\n</status>\nMake the customer's next move."
}
