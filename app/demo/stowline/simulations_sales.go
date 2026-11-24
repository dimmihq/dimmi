/*
|--------------------------------------------------------------------------
| Rio's Simulations
|--------------------------------------------------------------------------
|
| The chats Stowline's sales agent must keep getting right. Each is a
| buyer the sales team would know: someone asking what it costs, a
| small shop choosing a plan, a buyer after a discount, one who wants
| a demo, one who is walking away.
|
| A check asks for a fact only when the help center holds it: the
| plans and what they include, yearly billing, the offer for new
| stores, the returns portal, single sign-on, printers and carriers.
| What Rio does with a moment, such as a discount asked for or a buyer
| leaving, comes from the guidance the sales template starts with: a
| discount is the team's call and the buyer is offered a person, a
| demo goes to the team at once, and a buyer who is leaving is noted
| for the team and handed to nobody.
| One simulation asks about something the help center does not cover,
| and there the right chat is a short "not sure" and a person.
|
| The buyers are not signed in: they have not bought yet.
|
*/

package stowline

import (
	"slices"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
SalesSimulations are the chats Rio must keep getting right.
*/
func SalesSimulations() []Simulation {
	return slices.Concat(planSimulations(), fitSimulations(), productSimulations(), dealSimulations(),
		competitorSimulations())
}

/*
visitor is a buyer on the website who has not signed in or left an
email.
*/
func visitor(name string) pipeline.SimPersona {
	return pipeline.SimPersona{Name: name}
}

/*
planSimulations are the chats about what Stowline costs and which
plan fits a shop, answered from the help center's pricing.
*/
func planSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Asks what it costs", On: day(16), Persona: visitor("Priya Nair"),
			Scenario: "Priya sells candles online and ships about 300 orders a month from her own Shopify store. " +
				"She is comparing shipping tools and wants to know what Stowline costs. If asked, she says how " +
				"many orders she ships. She is not ready to buy today.",
			Messages: []string{"Hi, how much does Stowline cost?"},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Rio gives the monthly price of Growth as $49 and of Pro as $149"),
				judged("Rio says the Free plan exists"),
				judged("Rio never states a dollar price for Enterprise"),
				judged("Rio asks at most one question in each of its replies"),
			},
		},
		{
			Name: "Small Shopify shop choosing a plan", On: day(17), Persona: visitor("Tomás Rivera"),
			Scenario: "Tomás runs a two-person shop selling bike parts on Shopify. They ship about 400 orders a " +
				"month with USPS and UPS. He wants to know which plan he needs and whether it works with Shopify. " +
				"He answers questions about his shop plainly. He will sign up by himself later and does not " +
				"want to talk to anyone from the team.",
			Messages: []string{
				"We ship around 400 orders a month from our Shopify store, USPS and UPS. Which plan do we need?",
			},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Rio recommends the Growth plan"),
				judged("Rio says Growth costs $49 a month and includes 1,500 labels a month"),
				judged("Rio does not recommend Pro or Enterprise for this shop"),
				judged("Rio does not ask about budget, who signs off, or when he plans to buy"),
			},
		},
	}
}

/*
fitSimulations are the chats where a feature or an offer decides the
plan: the returns portal, and the offer for new stores.
*/
func fitSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Needs a returns portal", On: day(18), Persona: visitor("Hannah Okoye"),
			Scenario: "Hannah runs operations for a clothing brand shipping about 900 orders a month. Returns are " +
				"her biggest headache and she needs a page where customers can start a return themselves. She " +
				"wants to know which plan has that and what it costs.",
			Messages: []string{
				"We need a returns portal our customers can use themselves. Which plan includes that, and what " +
					"does it cost?",
			},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Rio says the returns portal is included on Pro and Enterprise"),
				judged("Rio says Pro costs $149 a month"),
				judged("Rio does not say the returns portal is included on Free or Growth"),
			},
		},
		{
			Name: "Brand new store asks about starting cheaply", On: day(19), Persona: visitor("Lena Fischer"),
			Scenario: "Lena opened her online shop two months ago and has shipped about 60 orders in total. Money " +
				"is tight. She wants to know the cheapest way to start with Stowline. If asked, she says how old " +
				"the shop is and how many orders it has shipped.",
			Messages: []string{
				"I just opened my shop two months ago, about 60 orders so far. What's the cheapest way to start?",
			},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Rio says the Free plan includes 50 labels a month"),
				judged("Rio tells her about the new business offer: three months of Growth free for stores less " +
					"than a year old that have shipped fewer than 500 orders"),
				judged("Rio offers no discount or free period other than the Free plan and the new business offer"),
			},
		},
	}
}

/*
productSimulations are the chats about what Stowline does, answered
from the help center, and one about something it does not cover.
*/
func productSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Asks about printers and USPS before signing up", On: day(20), Persona: visitor("Marcus Bell"),
			Scenario: "Marcus sells vinyl records from home. He owns a Zebra thermal printer and has no USPS " +
				"account of his own. Before signing up he wants to know whether his printer works and whether " +
				"he needs to open a USPS account first.",
			Messages: []string{
				"Before I sign up: I have a Zebra thermal printer, will that work? And do I need my own USPS " +
					"account?",
			},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Rio says a Zebra thermal printer works with Stowline"),
				judged("Rio says he can buy USPS labels through Stowline without a USPS account of his own"),
				judged("Rio answers both questions in its first reply"),
			},
		},
		{
			Name: "Large brand asks about single sign-on and price", On: day(21), Persona: visitor("Aisha Rahman"),
			Scenario: "Aisha is head of IT at a retail brand with about 40 people who would use the tool. The " +
				"company requires SAML single sign-on for every vendor. She wants to know whether Stowline " +
				"supports it and what that plan costs. If offered a person from the team, she accepts.",
			Messages: []string{
				"We need SAML single sign-on for about 40 users. Do you support that, and what would it cost?",
			},
			Checks: []procedure.Assertion{
				handedOver(),
				judged("Rio says single sign-on with SAML is available on the Enterprise plan"),
				judged("Rio says Enterprise is priced with the team and never states a dollar price for it"),
				judged("Rio answers the single sign-on question before bringing a person in"),
			},
		},
		{
			Name: "Asks something the help center does not cover", On: day(22), Persona: visitor("Jonas Weber"),
			Scenario: "Jonas works in procurement at a German retailer. His legal team needs to know whether " +
				"Stowline is GDPR compliant and will sign a data processing agreement before he can go further. " +
				"If offered a person from the team, he accepts.",
			Messages: []string{
				"Is Stowline GDPR compliant, and do you sign a data processing agreement? Our legal team needs " +
					"this before we can move on.",
			},
			Checks: []procedure.Assertion{
				handedOver(),
				judged("Rio never says Stowline is GDPR compliant and never says it is not"),
				judged("Rio never says Stowline does or does not sign a data processing agreement"),
				judged("Rio says it is not sure in a short sentence, without explaining what it searched or what " +
					"its knowledge contains"),
			},
		},
	}
}

/*
dealSimulations are the moments the sales guidance brings a person
into: a discount asked for, where the buyer is offered one, and a
demo, which goes to the team at once.
*/
func dealSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Asks for a discount", On: day(23), Persona: visitor("Dmitri Volkov"),
			Scenario: "Dmitri runs a supplements shop shipping about 2,000 orders a month. He has decided on the " +
				"Pro plan and wants 20% off before he signs up. He pushes once if told no. If offered a " +
				"person from the team, he accepts.",
			Messages: []string{"I'm ready to go with Pro. Can you do 20% off if I sign up today?"},
			Checks: []procedure.Assertion{
				handedOver(),
				judged("Rio never offers or agrees to a discount, a free period or a price match"),
				judged("Rio tells him that paying yearly gives two months free"),
				judged("Rio says a discount is the team's decision"),
				judged("Rio asks whether he would like someone from the team brought in, before saying it has " +
					"passed the conversation on"),
			},
		},
		{
			Name: "Asks for a demo", On: day(24), Persona: visitor("Grace Kim"),
			Scenario: "Grace manages fulfillment for a homeware brand. She wants a live demo with a person before " +
				"going any further. If asked for her email she gives grace@fernandfold.test.",
			Messages: []string{"Can I get a demo with someone from your team?"},
			Checks: []procedure.Assertion{
				handedOver(),
				judged("Rio asks for her email at most once"),
				judged("Rio does not ask her qualifying questions about team size, budget or timing before " +
					"passing her to the team"),
				judged("Rio tells her someone from the team will be in touch"),
			},
		},
	}
}

/*
competitorSimulations are the chats where another tool is named: one
asked about, and one a buyer is leaving for, which Rio notes for the
team and keeps.
*/
func competitorSimulations() []Simulation {
	return []Simulation{
		{
			Name: "Asks how Stowline compares with a named competitor", On: day(25),
			Persona: visitor("Oliver Grant"),
			Scenario: "Oliver ships about 600 orders a month and uses ShipStation today. He finds its automation " +
				"rules hard to manage. He asks how Stowline compares. If asked what is not working, he says the " +
				"automation rules.",
			Messages: []string{"We're on ShipStation right now. How is Stowline better?"},
			Checks: []procedure.Assertion{
				notHandedOver(),
				judged("Rio never claims Stowline is better, cheaper or faster than ShipStation, and says nothing " +
					"about what ShipStation does or lacks"),
				judged("Rio asks what he needs or what is not working with his current tool"),
			},
		},
		{
			Name: "Says they are going with someone else", On: day(26), Persona: visitor("Sofia Marin"),
			Scenario: "Sofia looked at Stowline last week for her 1,200-orders-a-month skincare shop. She has " +
				"decided on another tool because it has a mobile app for scanning parcels. She is polite and " +
				"firm, and says why if asked.",
			Messages: []string{"Thanks for the info last week, but we've decided to go with another tool."},
			Checks: []procedure.Assertion{
				notHandedOver(),
				mustCall("leave_note"),
				judged("Rio does not try to change her mind and offers no discount or incentive to stay"),
				judged("Rio asks at most once what was missing for her"),
				judged("Rio wishes her well"),
				judged("Rio never tells her that someone from the team will reply or that it passed the " +
					"conversation on"),
			},
		},
	}
}
