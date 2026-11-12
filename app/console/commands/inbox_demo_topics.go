//go:build dev

/*
|--------------------------------------------------------------------------
| Inbox Demo Topics
|--------------------------------------------------------------------------
|
| The people and the stories the inbox demo plays out: what customers
| write in about, how a teammate answers and how the customer replies.
|
*/

package commands

/*
demoName returns a believable name for the nth person.
*/
func demoName(n int) string {
	first := []string{
		"Ada", "Bola", "Chen", "Dara", "Eli", "Fatima", "Gus", "Hana", "Ivan", "Jade", "Kofi", "Lena",
		"Mateo", "Nia", "Omar", "Priya", "Quinn", "Rosa", "Sven", "Tara", "Uche", "Vera", "Wes", "Yara", "Zane",
	}
	last := []string{
		"Okafor", "Lindqvist", "Moreau", "Tanaka", "Silva", "Novak", "Haddad", "Kowalski", "Mensah",
		"Reyes", "Fischer", "Ibrahim", "Larsen", "Costa", "Nakamura", "Adeyemi",
	}

	return first[(n*7)%len(first)] + " " + last[(n*5+3)%len(last)]
}

/*
demoTopic is what one conversation is about and how it goes.
*/
type demoTopic struct {
	subject, opening, note string
	replies, answers       []string
}

var demoTopics = []demoTopic{
	{
		subject: "Can't sign in after resetting my password",
		opening: "Hi, I reset my password this morning but the new one is refused. Can you help?",
		note:    "Their account has two-factor on; the reset did not clear the old sessions.",
		replies: []string{
			"Sorry about that! Could you try signing in from a private window so old cookies don't get in the way?",
			"Thanks for trying. I've cleared your old sessions on our side, please try once more.",
			"Got it. Which browser and version are you on?",
		},
		answers: []string{
			"Still refused, same error.", "Chrome 131 on a Mac.", "That worked, thank you!",
			"Hmm, it signed me out again after a minute.",
		},
	},
	{
		subject: "Invoice shows the wrong company name",
		opening: "Our October invoice still shows our old company name. We need it corrected for accounting.",
		note:    "Billing profile was updated after the invoice was issued.",
		replies: []string{
			"Thanks for flagging! What should the company name and address read?",
			"I've updated the billing profile. I'll reissue October's invoice now.",
			"The corrected invoice is on its way to your billing email.",
		},
		answers: []string{
			"It should be Northwind Labs GmbH, Torstraße 12, Berlin.", "Received, looks right now.",
			"Could you also add our VAT number DE123456789?",
		},
	},
	{
		subject: "Webhook deliveries failing with 401",
		opening: "Since yesterday every webhook to our endpoint fails with 401. Did something change?",
		note:    "They rotated their signing secret without updating the verifier.",
		replies: []string{
			"Nothing changed on our side. Did your team rotate the signing secret recently?",
			"The old secret keeps signing for 24 hours after a rotation; after that only the new one works.",
			"Once your verifier has the new secret, I can resend the failed deliveries for you.",
		},
		answers: []string{
			"Yes, we rotated it on Monday.", "Updated it now, can you resend?", "All green, thanks a lot.",
		},
	},
	{
		subject: "Feature request: export conversations to CSV",
		opening: "Is there a way to export all our conversations from last quarter to a spreadsheet?",
		note:    "Customer is on Pro; exports are available from the list's menu.",
		replies: []string{
			"Yes! From any list, open the menu and pick Export; you can choose CSV.",
			"The file includes every message and note unless you untick notes.",
			"Large exports take a minute; you'll get an email when the file is ready.",
		},
		answers: []string{"Found it, thanks!", "Can it include tags too?", "Perfect, that's what we needed."},
	},
	{
		subject: "Messenger not showing on our docs site",
		opening: "We added the snippet to our docs but the chat bubble never appears.",
		note:    "Their CSP blocks the widget host.",
		replies: []string{
			"Could you open the browser console on the docs site and send any errors you see?",
			"That's your Content-Security-Policy blocking our script. Add our host to script-src and connect-src.",
			"After the CSP change, a hard refresh should show the bubble.",
		},
		answers: []string{
			"Refused to load the script because it violates the CSP directive.", "We updated the CSP.",
			"It shows now, great.",
		},
	},
}
