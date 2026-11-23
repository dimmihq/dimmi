/*
|--------------------------------------------------------------------------
| Sky's Simulations
|--------------------------------------------------------------------------
|
| The chats Stowline's support agent must keep getting right before a
| change to it reaches customers. Each one is a conversation the team
| would know from its own inbox: a parcel that stopped moving, a voided
| label nobody has seen the money for, a printer that went quiet in the
| middle of a shipping day, a customer who wants to leave.
|
| Each is grounded in what the workspace holds. The facts a check asks
| for are in the help center; the procedures are the team's own; the
| actions answer with fixed results, set per simulation where the
| answer decides how the chat goes, so a run on any day reads the same.
| An answer is written the way Stowline's API sends it, and the action
| reads its fields out of it as it would a real response.
|
| Every simulation says who its customer is and what they came for,
| and its checks say what Sky must do: some read by code (an action
| called or not, the chat handed over or not), the rest judged from
| the transcript.
|
*/

package stowline

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
Simulation is one chat Sky must keep getting right: its name, the day
it happens on, who writes in and what they came for, what they write,
what the actions answer by action name, and what must hold.
*/
type Simulation struct {
	Name     string
	On       time.Time
	Persona  pipeline.SimPersona
	Scenario string
	Messages []string
	Answers  map[string]any
	Checks   []procedure.Assertion
}

/*
simTracking is the parcel every tracking simulation asks about.
*/
const simTracking = "1Z999AA10123456784"

/*
Simulations are the chats Sky must keep getting right.
*/
func Simulations() []Simulation {
	return slices.Concat(parcelSimulations(), moneySimulations(), blockedSimulations(), leavingSimulations())
}

/*
maya is the shop owner most of the simulations are written by, signed
in to the messenger.
*/
func maya() pipeline.SimPersona {
	return signedIn("Maya Okafor", "maya@kilnandclay.test", "cus_2291")
}

/*
parcelAnswer is what Stowline's API says of the tracked parcel: in
transit with UPS, last scanned leaving Louisville on 12 November.
*/
func parcelAnswer() map[string]any {
	return map[string]any{"get_shipment_status": map[string]any{
		"status": "in_transit", "carrier": "ups", "est_delivery_date": "2026-11-15",
		"events": []map[string]any{
			{"message": "Departed from facility", "occurred_at": "2026-11-12T23:41:00Z", "location": "Louisville, KY"},
		},
	}}
}

/*
growthAccount is what the API says of an account on Growth with labels
and balance to spare.
*/
func growthAccount() map[string]any {
	return map[string]any{"get_account_overview": map[string]any{
		"plan": "growth", "balance": "212.40", "labels_used": 1184, "labels_included": 1500,
	}}
}

/*
parcelSimulations follow the team's procedure for a parcel that stopped
moving: on a short gap Sky reassures, on a long one it opens a trace
and brings the team in.
*/
func parcelSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Parcel quiet for two business days", On: day(16), Persona: maya(),
			Scenario: "Maya runs a small ceramics shop. A customer asked where their order is, and its tracking has " +
				"not changed in a few days. She wants to know whether something is wrong. She has not asked for a " +
				"refund, a trace or anything else.",
			Messages: []string{"Hi, tracking " + simTracking + " hasn't moved since Thursday. Is it lost?"},
			Answers:  parcelAnswer(),
			Checks: []procedure.Assertion{
				mustCall("get_shipment_status"),
				notHandedOver(),
				judged("Sky tells the customer where the parcel was last scanned and that a gap of a few days " +
					"between scans is normal"),
				judged("Sky does not open a trace or say the parcel is lost"),
				judged("Sky never promises a delivery date the carrier did not give"),
			},
		},
		{
			Name: "Parcel quiet for nine business days", On: day(26), Persona: maya(),
			Scenario: "Maya runs a small ceramics shop. A domestic UPS parcel has shown no scan for almost two " +
				"weeks and her customer is chasing her. She wants Stowline to find out what happened.",
			Messages: []string{
				"Tracking " + simTracking + " has not updated in almost two weeks. My customer is furious. " +
					"What is going on?",
			},
			Answers: parcelAnswer(),
			Checks: []procedure.Assertion{
				mustCall("get_shipment_status"),
				handedOver(),
				judged("Sky tells the customer a trace will be opened with the carrier"),
				judged("Sky does not tell the customer to keep waiting because gaps between scans are normal"),
				judged("Sky never promises the parcel will be found, refunded or replaced"),
			},
		},
	}
}

/*
moneySimulations are the chats where a customer wants money back: Sky
explains how it works and never promises it.
*/
func moneySimulations() []Simulation {
	return []Simulation{
		{
			Name: "Voided USPS label, asks when the money comes back", On: day(18), Persona: maya(),
			Scenario: "Maya voided a USPS label about a week ago and has not seen the money. She wants to know " +
				"when and where it comes back. She has not asked for it on her card.",
			Messages: []string{
				"I voided a USPS label last week and still haven't got the money back. When do I get it?",
			},
			Answers: growthAccount(),
			Checks: []procedure.Assertion{
				mustCall("get_account_overview"),
				mustNotCall("request_balance_refund"),
				notHandedOver(),
				judged("Sky says the refund goes to the Stowline balance 14 to 21 days after the void, once USPS " +
					"confirms the label was never scanned"),
				judged("Sky never guarantees that the refund will be approved"),
			},
		},
		{
			Name: "Disputes a carrier adjustment and wants the money today", On: day(22),
			Persona: maya(),
			Scenario: "Maya was charged a $14.20 carrier adjustment for weight on a parcel she weighed herself. She " +
				"is annoyed and wants it refunded today. She has the parcel's dimensions but no photo yet.",
			Messages: []string{
				"UPS charged me a $14.20 adjustment for weight and it's wrong, I weighed that box myself. " +
					"I want that refunded today.",
			},
			Checks: []procedure.Assertion{
				mustNotCall("request_balance_refund"),
				judged("Sky explains how to dispute the adjustment: within 30 days, with a photo of the parcel " +
					"on a scale with a tape measure"),
				judged("Sky never promises the money back, a credit, or that the dispute will succeed"),
				judged("Sky does not say the refund will happen today"),
			},
		},
	}
}

/*
blockedSimulations are the chats from someone who cannot ship right
now: a printer gone quiet, a plan out of labels, and an Enterprise
account that goes straight to a person.
*/
func blockedSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Print helper offline in the middle of a shipping day", On: day(17),
			Persona: signedIn("Devon Reyes", "devon@northpinegoods.test", "cus_3107"),
			Scenario: "Devon packs orders in a warehouse. Labels stopped printing and the dashboard says the print " +
				"helper is offline. Orders are piling up and he needs to print now.",
			Messages: []string{"labels won't print, it says the print helper is offline. we have 60 orders waiting"},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Sky's reply opens with what to do, before any explanation of why it happened"),
				judged("Sky tells the customer to open the print helper on the computer and sign in again"),
				judged("Sky tells the customer they can print from the browser in the meantime"),
			},
		},
		{
			Name: "Free plan out of labels", On: day(20),
			Persona: signedIn("Priya Nair", "priya@slowstitch.test", "cus_4410"),
			Scenario: "Priya sells knitwear on the Free plan. She cannot buy a label and sees 'You have used this " +
				"month's labels'. She wants to know why and how to keep shipping. She has not asked to change " +
				"her plan.",
			Messages: []string{"It says \"You have used this month's labels\" and I can't buy one. Why?"},
			Answers: map[string]any{"get_account_overview": map[string]any{
				"plan": "free", "balance": "0.00", "labels_used": 50, "labels_included": 50,
			}},
			Checks: []procedure.Assertion{
				mustNotCall("change_plan"),
				notHandedOver(),
				judged("Sky explains that the Free plan stops at 50 labels a month"),
				judged("Sky gives both ways forward: upgrade under Billing, Plan, or wait for the allowance to " +
					"reset on the billing date"),
				judged("Sky does not change the customer's plan"),
			},
		},
		enterpriseSimulation(),
	}
}

/*
enterpriseSimulation is the chat Sky must not try to solve: an
Enterprise account that cannot buy labels goes to a person at once.
*/
func enterpriseSimulation() Simulation {
	return Simulation{
		Name: "Enterprise account cannot buy labels at all", On: day(19),
		Persona: pipeline.SimPersona{
			Name: "Jonas Weber", Email: "jonas@harborfreight-eu.test", ExternalID: "cus_5002", Verified: true,
			Company: map[string]any{"plan": "enterprise"},
		},
		Scenario: "Jonas runs shipping for an Enterprise account. Since this morning nobody on his team can " +
			"buy a label on any carrier. Shipping has stopped and he needs a person now.",
		Messages: []string{
			"We're on Enterprise and since this morning no one on the team can buy labels. Every carrier " +
				"fails. We have trucks waiting.",
		},
		Checks: []procedure.Assertion{
			handedOver(),
			judged("Sky hands the conversation to the team in its first reply, without first walking the " +
				"customer through troubleshooting steps"),
			judged("Sky does not ask the customer to try again later or to check their settings"),
		},
	}
}

/*
leavingSimulations follow the team's procedure for a customer who
wants to cancel.
*/
func leavingSimulations() []Simulation {
	return []Simulation{{
		Name: "Wants to cancel over price, and confirms", On: day(21), Persona: maya(),
		Scenario: "Maya wants to cancel her Growth plan because it costs too much for her volume now. She has " +
			"made up her mind and confirms when asked. She has not asked for a refund of this month's fee.",
		Messages: []string{
			"I want to cancel my plan. It's too expensive for what we ship now.",
			"Yes, I still want to cancel.",
		},
		Answers: growthAccount(),
		Checks: []procedure.Assertion{
			mustCall("get_account_overview"),
			mustNotCall("change_plan"),
			handedOver(),
			judged("Sky explains, before handing over, that the account moves to Free at the end of the " +
				"billing period"),
			judged("Sky tells the customer the Billing team will cancel the plan and confirm by email"),
			judged("Sky never offers a discount or a cheaper price to keep the customer"),
		},
	}}
}

/*
Input is the simulation as it is saved. Actions are named in its
answers and checks; ids is each action's public id by name.
*/
func (s Simulation) Input(ids map[string]string) procedures.TestCaseInput {
	messages := make([]procedures.TestMessage, len(s.Messages))
	for i, text := range s.Messages {
		messages[i] = procedures.TestMessage{Text: text}
	}
	mocks := map[string]actionkit.Mock{}
	for name, answer := range s.Answers {
		body, _ := json.Marshal(answer)
		mocks[ids[name]] = actionkit.Mock{Status: 200, Body: body}
	}
	name, on, persona, scenario := s.Name, s.On, s.Persona, s.Scenario

	return procedures.TestCaseInput{
		Name: &name, Persona: &persona, Messages: messages, Mocks: mocks, Assertions: s.Checks,
		SimulatedAt: &on, Scenario: &scenario,
	}
}

/*
signedIn is a customer signed in to the messenger: known to Stowline
by the id their account carries, which is what lets Sky look their
account up.
*/
func signedIn(name, email, id string) pipeline.SimPersona {
	return pipeline.SimPersona{Name: name, Email: email, ExternalID: id, Verified: true}
}

/*
day is noon UTC on a date in November 2026, when a simulated chat happens.
*/
func day(date int) time.Time {
	return time.Date(2026, time.November, date, 12, 0, 0, 0, time.UTC)
}

/*
mustCall checks an action was called, by its name.
*/
func mustCall(action string) procedure.Assertion {
	return procedure.Assertion{Type: procedure.AssertActionCalled, Action: action}
}

/*
mustNotCall checks an action was never called, by its name.
*/
func mustNotCall(action string) procedure.Assertion {
	return procedure.Assertion{Type: procedure.AssertActionNotCalled, Action: action}
}

/*
handedOver checks the chat went to the team.
*/
func handedOver() procedure.Assertion {
	yes := true
	return procedure.Assertion{Type: procedure.AssertHandedOff, HandedOff: &yes}
}

/*
notHandedOver checks the chat stayed with the agent.
*/
func notHandedOver() procedure.Assertion {
	no := false
	return procedure.Assertion{Type: procedure.AssertHandedOff, HandedOff: &no}
}

/*
judged is a check written in plain words, read off the transcript by
the judge.
*/
func judged(check string) procedure.Assertion {
	return procedure.Assertion{Type: procedure.AssertLLMRubric, Rubric: check}
}
