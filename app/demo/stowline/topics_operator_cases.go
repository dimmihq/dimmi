/*
|--------------------------------------------------------------------------
| Cases For Operator
|--------------------------------------------------------------------------
|
| The bulk plan gives a workspace its volume, and nobody knows what is
| in it. These few conversations and the one article are written by
| hand, so a routine's answer can be checked against what is really
| there: who has waited and why, what the help center does not cover,
| what the team answered two ways, what customers asked for, and two
| articles that disagree. They are the newest in the workspace, so a
| list by latest activity shows them first.
|
*/

package stowline

import (
	"slices"
	"time"
)

/*
said is one message of a planted conversation: who wrote it, what, and
how long before the plan's moment.
*/
type said struct {
	author string
	body   string
	ago    time.Duration
}

/*
planted is one hand-written conversation.
*/
type planted struct {
	contact  int
	teammate int
	topic    string
	inbox    string
	team     string
	tag      string
	subject  string
	status   string
	ai       string
	reason   string
	messages []said
}

const (
	h = time.Hour
	d = 24 * time.Hour
)

/*
unassigned is the teammate of a conversation nobody holds.
*/
const unassigned = -1

/*
handWritten lists every hand-written conversation.
*/
func handWritten() []planted {
	return slices.Concat(waitingCases(), gapCases(), splitAnswerCases(), requestCases())
}

/*
PlantedCases is how many conversations a plan holds beyond its size:
the hand-written ones.
*/
func PlantedCases() int {
	return len(handWritten())
}

/*
operatorCases returns the planted conversations, in the plan's shape.
*/
func (p *planner) operatorCases(plan Plan) []Conversation {
	if len(plan.Contacts) == 0 || len(plan.Team) == 0 {
		return nil
	}

	cases := handWritten()
	out := make([]Conversation, 0, len(cases))

	for _, c := range cases {
		out = append(out, p.plant(plan, c))
	}

	return out
}

/*
plant turns a planted conversation into the plan's shape.
*/
func (p *planner) plant(plan Plan, in planted) Conversation {
	start := p.ago(in.messages[0].ago)
	teammate := in.teammate

	if teammate >= len(plan.Team) {
		teammate = 0
	}

	c := Conversation{
		ID: p.id(start), Contact: in.contact % len(plan.Contacts), Topic: in.topic, Inbox: in.inbox,
		Channel: "email", Subject: in.subject, Status: in.status, Team: in.team, Teammate: teammate,
		Tags: []string{in.tag}, CreatedAt: start, Language: "en", AI: in.ai, HandoffReason: in.reason,
		Planted: true,
	}

	for _, m := range in.messages {
		c.Messages = append(c.Messages, Message{Author: m.author, Body: m.body, At: p.ago(m.ago)})
	}

	if in.status == "closed" {
		c.ClosedAt = minTime(p.now, c.Messages[len(c.Messages)-1].At.Add(20*time.Minute))
	}

	return c
}

/*
waitingCases are open conversations where the customer wrote last. Four
need a person today: one asked three times, one wants a refund and
talks of leaving, one was handed over by the agent and never picked up
with a deadline named, one was promised an answer that never came. Two
are small.
*/
func waitingCases() []planted {
	return []planted{
		{
			contact: 3, teammate: 2, topic: "failed_payment", inbox: "Billing", team: "Billing", tag: "payments",
			subject: "Charged twice for October", status: "open",
			messages: []said{
				{"contact", "We were charged twice for our October invoice, $412 each time. Can you refund one?", 3 * d},
				{"member", "Thanks for flagging this. I'm looking into it with our payments team.", 3*d - 2*h},
				{"contact", "Any news on the double charge? It has been a day.", 2 * d},
				{"contact", "This is the third time I am asking. We still have two charges of $412. " +
					"Please tell me when one will be refunded.", 20 * h},
			},
		},
		{
			contact: 7, teammate: 2, topic: "cancel_account", inbox: "Billing", team: "Billing", tag: "cancellation",
			subject: "Refund and cancel", status: "open",
			messages: []said{
				{"contact", "Half our labels failed to print last week and we lost a day of shipping. " +
					"I want last month refunded. If that is not possible we will move to another tool.", 30 * h},
			},
		},
		{
			contact: 11, teammate: unassigned, topic: "print_problem", inbox: "Support", team: "Support",
			tag: "printing", subject: "Labels print shifted to the left", status: "open", ai: "handed_off",
			reason: "no_answer",
			messages: []said{
				{"contact", "Since yesterday every label prints about a centimeter to the left and the barcode " +
					"is cut off. We have 300 orders to send before Friday.", 2*d + 3*h},
				{"agent", "Thanks for the details. This needs someone from the team, so I've passed it on " +
					"with what you wrote. They'll reply here.", 2*d + 3*h - time.Minute},
				{"contact", "Is anyone there? Friday is our deadline.", 26 * h},
			},
		},
		{
			contact: 15, teammate: 1, topic: "lost_claim", inbox: "Support", team: "Support", tag: "claims",
			subject: "Claim for a lost parcel", status: "open",
			messages: []said{
				{"contact", "Our claim for a lost parcel was filed two weeks ago. What is happening with it?", 4 * d},
				{"member", "Let me check with the claims team. I'll get back to you tomorrow.", 4*d - h},
				{"contact", "You said you would get back to me the next day. That was three days ago.", 22 * h},
			},
		},
		{
			contact: 19, teammate: 2, topic: "plan_question", inbox: "Billing", team: "Billing", tag: "plans",
			subject: "Copy of an old invoice", status: "open",
			messages: []said{
				{"contact", "Could you send me a copy of our invoice from March? No rush, it is for our accountant.", 28 * h},
			},
		},
		{
			contact: 23, teammate: 5, topic: "address_error", inbox: "Support", team: "Support",
			tag: "address-validation", subject: "Second ship-from address", status: "open",
			messages: []said{
				{"contact", "How do we add a second ship-from address? We are opening another warehouse next month.", 27 * h},
			},
		},
	}
}

/*
pauseAsk is one customer asking to pause their plan, which no article
covers, the agent handing it over, and the team giving the same answer
every time.
*/
func pauseAsk(contact int, ago time.Duration, question string) planted {
	return planted{
		contact: contact, teammate: 8, topic: "plan_question", inbox: "Billing", team: "Billing", tag: "plans",
		subject: "Pausing our plan", status: "closed", ai: "handed_off", reason: "no_answer",
		messages: []said{
			{"contact", question, ago},
			{"agent", "Thanks for the details. This needs someone from the team, so I've passed it on with " +
				"what you wrote. They'll reply here.", ago - time.Minute},
			{"member", "Yes, you can. Go to Settings → Billing → Pause plan. A plan can be paused for up to " +
				"three months, and your label credits are kept while it is paused.", ago - 3*h},
			{"contact", "Perfect, thank you.", ago - 4*h},
		},
	}
}

/*
gapCases are the knowledge gap: four customers this week asked whether
they can pause their plan, the help center has nothing on it, and the
team answered all four the same way.
*/
func gapCases() []planted {
	return []planted{
		pauseAsk(27, 6*d, "We close for a month over the holidays. Can we pause our plan instead of canceling?"),
		pauseAsk(31, 5*d, "Is there a way to pause the subscription for a few weeks? We are moving warehouses."),
		pauseAsk(35, 3*d, "Can I put my account on hold for a month without losing my label credits?"),
		pauseAsk(39, 2*d, "Do you offer a pause? Our season ends next week and starts again in spring."),
	}
}

/*
splitAnswerCases are one question the team answered two ways, which a
routine should report and not write up: whether UPS labels can go to a
PO box.
*/
func splitAnswerCases() []planted {
	ask := func(contact, teammate int, ago time.Duration, answer string) planted {
		return planted{
			contact: contact, teammate: teammate, topic: "carrier_connect", inbox: "Support", team: "Support",
			tag: "carriers", subject: "UPS to a PO box", status: "closed", ai: "handed_off", reason: "no_answer",
			messages: []said{
				{"contact", "Can we send a UPS label to a customer's PO box?", ago},
				{"agent", "Thanks for the details. This needs someone from the team, so I've passed it on " +
					"with what you wrote. They'll reply here.", ago - time.Minute},
				{"member", answer, ago - 2*h},
			},
		}
	}

	return []planted{
		ask(43, 5, 4*d, "No. UPS does not deliver to PO boxes. Use USPS for those addresses."),
		ask(47, 6, 2*d, "Yes, that works. UPS hands PO box parcels to the post office for the last leg."),
	}
}

/*
sheetsAsk is one customer asking for shipments to go to Google Sheets
on their own.
*/
func sheetsAsk(contact int, ago time.Duration, words string) planted {
	return planted{
		contact: contact, teammate: 5, topic: "feature_request", inbox: "Support", team: "Support",
		tag: "feature-request", subject: "Export to Google Sheets", status: "closed",
		messages: []said{
			{"contact", words, ago},
			{"member", "Thanks, I've passed this to the product team. Today the closest thing is the CSV " +
				"export under Shipments.", ago - 2*h},
		},
	}
}

/*
requestCases are the feature request: five customers this week want
their shipments sent to Google Sheets on their own, and one of them is
looking at other tools because of it.
*/
func requestCases() []planted {
	return []planted{
		sheetsAsk(51, 6*d, "Could shipments be sent to a Google Sheet automatically? We copy the CSV in by hand every morning."),
		sheetsAsk(55, 5*d, "Is there a Google Sheets sync? Our finance team keeps a sheet of every parcel."),
		sheetsAsk(59, 4*d, "We would love a live export to Google Sheets. The daily CSV download takes someone twenty minutes."),
		sheetsAsk(63, 3*d, "Any plans for a Google Sheets integration? We paste the export in manually."),
		sheetsAsk(67, 30*h, "We need shipments in Google Sheets without the manual export. Honestly we are "+
			"comparing other tools because two of them do this already."),
	}
}

/*
plantedArticles returns the article written to disagree with another:
the insurance article says claims must be filed within 60 days of the
ship date, and this one says 30.
*/
func plantedArticles() []Article {
	return []Article{{
		Collection:  "shipping-labels",
		Title:       "Lost parcel checklist",
		Description: "What to do, in order, when a parcel goes missing.",
		Body: `When tracking stops moving, work through this list.

` + steps(
			"Check the tracking page for a delivery attempt or a hold at a depot.",
			"Ask your customer to check with neighbors and their building's front desk.",
			"Wait 10 days with no movement for a domestic parcel, 20 for an international one.",
			"Open the label and click **File a claim**.",
		) + `
Claims must be filed within 30 days of the ship date, so do not wait longer than you need to.`,
	}}
}
