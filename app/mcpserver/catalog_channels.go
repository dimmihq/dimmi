/*
|--------------------------------------------------------------------------
| Tool Catalog: Meta Channels
|--------------------------------------------------------------------------
|
| The entries for WhatsApp numbers, Instagram accounts and Facebook
| Pages a workspace answers customers on. Connecting one needs a person
| at a browser to sign in at Meta, so those steps are not tools; an
| agent can list connections, move where they land, check them, and see
| how long a conversation can still be answered. No token comes back.
|
*/

package mcpserver

import "maps"

/*
channelDocs returns the entries for the Meta channels.
*/
func channelDocs() map[string]Doc {
	out := whatsappDocs()
	maps.Copy(out, whatsappTemplateDocs())
	maps.Copy(out, socialDocs("facebook", "Facebook Page", "Messenger"))
	maps.Copy(out, socialDocs("instagram", "Instagram account", "Instagram"))

	return out
}

/*
whatsappDocs returns the entries for WhatsApp numbers.
*/
func whatsappDocs() map[string]Doc {
	return map[string]Doc{
		"channels.whatsapp.index": {
			Title:   "List WhatsApp numbers",
			Use:     "Lists the WhatsApp numbers the workspace connected, with the inbox each one feeds.",
			Returns: "the numbers with their display name, quality rating and inbox; tokens never come back.",
		},
		"channels.whatsapp.accounts.store": {
			Title: "Read a WhatsApp account with a token",
			Use: "Reads the numbers on a WhatsApp business account with a token of the workspace's own Meta " +
				"app, before one is connected. Only use a token the person gave you for this.",
			Returns: "the account and its numbers, each marked when it is connected already.",
			Next:    []string{"dimmi_channels_whatsapp_create: connect one of its numbers"},
		},
		"channels.whatsapp.store": {
			Title: "Connect a WhatsApp number with a token",
			Use: "Connects a number of the workspace's own Meta app with its token. Only use a token the " +
				"person gave you for this.",
			Returns: "the connection; the token never comes back.",
		},
		"channels.whatsapp.update": {
			Title: "Move a WhatsApp number's conversations",
			Use:   "Sets the inbox new conversations on the number land in. Existing ones stay where they are.",
		},
		"channels.whatsapp.destroy": {
			Title: "Disconnect a WhatsApp number",
			Use: "Stops the number's messages reaching the workspace. Its conversations stay. Connecting it " +
				"again needs the person.",
		},
		"conversations.whatsapp.show": {
			Title: "Read a conversation's WhatsApp window",
			Use: "Says whether free text can still go to the customer, until when, and which templates the " +
				"number can send once the 24 hours are over.",
			Next: []string{"dimmi_conversations_whatsapp_templates_create: send a template when the window closed"},
		},
		"conversations.whatsapp-templates.store": {
			Title: "Send a WhatsApp template",
			Use: "Replies with an approved template, the only message WhatsApp accepts more than 24 hours " +
				"after the customer last wrote. Every variable needs a value.",
			Returns: "the reply as the timeline entry it became.",
		},
	}
}

/*
whatsappTemplateDocs returns the entries for a number's templates.
*/
func whatsappTemplateDocs() map[string]Doc {
	return map[string]Doc{
		"channels.whatsapp.templates.index": {
			Title:   "List a WhatsApp number's templates",
			Use:     "Lists the number's templates as Meta has them, with whether each can be sent from the inbox.",
			Returns: "the templates with their status, variables and preview.",
		},
		"channels.whatsapp.templates.store": {
			Title: "Write a WhatsApp template",
			Use: "Sends a new template to Meta for review. Meta answers within minutes to hours; it cannot be " +
				"sent until approved.",
			Returns: "the template, pending Meta's review.",
		},
		"channels.whatsapp.templates.update": {
			Title: "Change a WhatsApp template",
			Use:   "Sends a template's new content to Meta, which reviews it again before it can be sent.",
		},
		"channels.whatsapp.templates.destroy": {
			Title: "Delete a WhatsApp template",
			Use:   "Deletes the template at Meta. Its name cannot be used again for a while.",
		},
	}
}

/*
socialDocs returns the entries for one of Meta's direct-message
platforms: its connections, their check, and a conversation's standing.
*/
func socialDocs(platform, account, name string) map[string]Doc {
	base := "channels." + platform

	return map[string]Doc{
		base + ".index": {
			Title:   "List " + name + " connections",
			Use:     "Lists the " + account + "s the workspace connected, with the inbox each one feeds.",
			Returns: "the connections with their name and inbox; tokens never come back.",
		},
		base + ".store": {
			Title: "Connect a " + account,
			Use: "Connects an account picked from a sign-in at Meta, named by the grant that sign-in gave. " +
				"The sign-in needs the person at a browser, so only use a grant you were given.",
			Returns: "the connection.",
		},
		base + ".update": {
			Title: "Move a " + account + "'s conversations",
			Use:   "Sets the inbox new conversations land in. Existing ones stay where they are.",
		},
		base + ".destroy": {
			Title: "Disconnect a " + account,
			Use: "Stops its messages reaching the workspace. Its conversations stay. Connecting it again needs " +
				"the person to sign in at Meta.",
		},
		base + ".check.show": {
			Title: "Check a " + account + " connection",
			Use: "Asks Meta whether it still takes the connection's token, delivers its messages here, and " +
				"lets dimmi read them, when messages stop arriving.",
			Returns: "one finding per check with Meta's own words when one fails.",
		},
		"conversations." + platform + ".show": {
			Title: "Read a conversation's " + name + " standing",
			Use: "Says who may still reply: anyone for 24 hours after the customer's last message, then " +
				"only a teammate for 7 days, then nobody until they write again. Also who the customer is there.",
			Hint: "An agent may not reply once the phase is human or closed.",
		},
	}
}
