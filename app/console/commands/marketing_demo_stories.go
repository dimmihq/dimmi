//go:build dev

/*
|--------------------------------------------------------------------------
| Marketing Demo: Team And Stories
|--------------------------------------------------------------------------
|
| The people and conversations the marketing demo seeds: five named
| teammates and a dozen conversations written the way real customers
| write, newest first.
|
*/

package commands

import "time"

/*
marketingTeammates returns the people on the demo team, in the order the
stories hand conversations to them.
*/
func marketingTeammates() []string {
	return []string{"Priya Moreau", "Kofi Mensah", "Lena Fischer", "Mateo Reyes", "Hana Tanaka"}
}

/*
marketingTurn is one message in a story: who says it and what.
*/
type marketingTurn struct {
	by   string // One of "customer", "agent" or "note".
	body string
}

/*
marketingStory is one conversation: who wrote in, through which
channel, what about, how it went, where it stands and how long ago it
last moved.
*/
type marketingStory struct {
	name, company, channel, subject string
	turns                           []marketingTurn
	status                          string // One of open, closed or snoozed.
	teammate                        int    // Index into marketingTeammates, -1 for nobody.
	ago                             time.Duration
}

/*
marketingStories returns the demo's conversations, newest first.
*/
func marketingStories() []marketingStory {
	stories := []marketingStory{marketingWebhookStory()}
	stories = append(stories, marketingRecentStories()...)
	stories = append(stories, marketingMorningStories()...)

	return append(stories, marketingEarlierStories()...)
}

/*
marketingWebhookStory is the newest story, the one the inbox shot opens
on: a webhook outage found and fixed, with a note between teammates.
*/
func marketingWebhookStory() marketingStory {
	return marketingStory{
		name: "Omar Haddad", company: "Northwind Labs", channel: "email",
		subject: "Webhook deliveries failing since 9am",
		turns: []marketingTurn{
			{"customer", "Hi team, every webhook to our endpoint has failed since about 9:00 UTC. " +
				"Nothing changed on our side. Can you check what you see?"},
			{"note", "Their endpoint answers 301 to our requests since they moved to a new domain. " +
				"Our sender doesn't follow redirects."},
			{"agent", "Thanks Omar, I can see it. Since 08:57 UTC your endpoint answers every delivery " +
				"with a 301 redirect to api.northwind.dev, and webhooks don't follow redirects. " +
				"If you update the endpoint URL in Settings → Webhooks, " +
				"the failed deliveries will retry on their own within the hour."},
			{"customer", "That's it, we moved domains last night. " +
				"Updated now, I can see deliveries coming through again. Thank you!"},
		},
		status: "open", teammate: 0, ago: 3 * time.Minute,
	}
}

/*
marketingRecentStories returns the stories from the last hour, after the
webhook one.
*/
func marketingRecentStories() []marketingStory {
	return []marketingStory{
		{
			name: "Hana Kowalski", company: "Fieldnote", channel: "widget",
			subject: "Adding a teammate to our workspace",
			turns: []marketingTurn{
				{"customer", "How do I add a new teammate? I can't find the invite button."},
				{"agent", "Open Settings, then Team, and choose Invite. " +
					"They get an email link that's good for 7 days."},
			},
			status: "open", teammate: 1, ago: 11 * time.Minute,
		},
		{
			name: "Sven Larsen", company: "Brightwave", channel: "email",
			subject: "SSO broken after certificate rotation",
			turns: []marketingTurn{
				{"customer", "We rotated our SAML signing certificate this morning " +
					"and now nobody on the team can sign in."},
			},
			status: "open", teammate: -1, ago: 24 * time.Minute,
		},
		{
			name: "Ada Okafor", company: "Pinecrest", channel: "widget", subject: "Charged twice for November",
			turns: []marketingTurn{
				{"customer", "Looks like we were charged twice for November. Can you refund one of them?"},
				{"agent", "You're right, there are two $49.00 charges on November 3. " +
					"I've refunded the duplicate; it shows on your card in 5 to 10 days."},
				{"customer", "Perfect, thanks for the quick fix."},
			},
			status: "closed", teammate: 2, ago: 38 * time.Minute,
		},
		{
			name: "Mateo Silva", company: "Larkspur", channel: "api", subject: "Rate limits on the contacts API",
			turns: []marketingTurn{
				{"customer", "We're importing about 80k contacts and keep hitting 429s. " +
					"What's the limit and is there a bulk endpoint?"},
			},
			status: "open", teammate: 3, ago: 52 * time.Minute,
		},
	}
}

/*
marketingMorningStories returns the stories from earlier this morning.
*/
func marketingMorningStories() []marketingStory {
	return []marketingStory{
		{
			name: "Yara Ibrahim", company: "Quill & Co", channel: "email", subject: "Export all conversations to CSV?",
			turns: []marketingTurn{
				{"customer", "Is there a way to export every conversation from last quarter to CSV for our QA review?"},
				{"agent", "Not as a CSV yet, but it's on our board. " +
					"I've added your vote, and you'll get an email as soon as it ships."},
			},
			status: "snoozed", teammate: 4, ago: 75 * time.Minute,
		},
		{
			name: "Wes Novak", company: "Tidewater", channel: "widget", subject: "Messenger not showing on mobile",
			turns: []marketingTurn{
				{"customer", "The chat bubble shows on desktop but not on our mobile site. Any ideas?"},
				{"agent", "Check Settings → Messenger → Visibility: mobile is switched off for your site. " +
					"Turn it on and it appears right away."},
			},
			status: "closed", teammate: 0, ago: 2 * time.Hour,
		},
		{
			name: "Tara Mensah", company: "Oakline", channel: "email", subject: "Custom domain for our help center",
			turns: []marketingTurn{
				{"customer", "We'd like our help center on help.oakline.io. What DNS records do we need?"},
			},
			status: "open", teammate: 1, ago: 3 * time.Hour,
		},
		{
			name: "Eli Nakamura", company: "Northwind Labs", channel: "api",
			subject: "Identity verification with Next.js",
			turns: []marketingTurn{
				{"customer", "Following the Next.js guide, " +
					"but the messenger still shows visitors as anonymous after sign-in."},
				{"agent", "Your token route returns the token as plain text; " +
					"the messenger expects JSON like {\"token\": \"...\"}. Wrapping it fixes it."},
			},
			status: "closed", teammate: 3, ago: 5 * time.Hour,
		},
	}
}

/*
marketingEarlierStories returns the oldest stories, from today and
yesterday.
*/
func marketingEarlierStories() []marketingStory {
	return []marketingStory{
		{
			name: "Rosa Costa", company: "Fieldnote", channel: "widget", subject: "Office hours in two time zones",
			turns: []marketingTurn{
				{"customer", "Our team is split between Lisbon and Toronto. Can office hours cover both?"},
			},
			status: "open", teammate: -1, ago: 7 * time.Hour,
		},
		{
			name: "Ivan Adeyemi", company: "Brightwave", channel: "email", subject: "Upgrade to annual billing",
			turns: []marketingTurn{
				{"customer", "We'd like to move to annual billing from next month. Can you set that up?"},
				{"agent", "Done. Annual billing starts on your next renewal, December 1, " +
					"and your invoice goes to billing@brightwave.com."},
			},
			status: "closed", teammate: 2, ago: 26 * time.Hour,
		},
		{
			name: "Nia Lindqvist", company: "Pinecrest", channel: "widget", subject: "Translate articles into German",
			turns: []marketingTurn{
				{"customer", "Can our help center serve German versions of the articles?"},
			},
			status: "open", teammate: 4, ago: 30 * time.Hour,
		},
	}
}
