/*
|--------------------------------------------------------------------------
| Sample Catalog
|--------------------------------------------------------------------------
|
| The parts of the sample that are not prose: the help centers with their
| brand colors, the people who write, the collections articles are filed
| in and the labels changes are filed under. Collections nest one level,
| named by their parent's slug.
|
*/

package helpsample

import (
	"github.com/dimmihq/dimmi/resources/views/changelog"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
site is a sample help center and the slug it is found by.
*/
type site struct {
	slug   string
	center helpcenter.Center
}

/*
collection is a sample collection; parent is empty for a top-level one.
*/
type collection struct {
	slug        string
	parent      string
	title       string
	description string
	icon        string
}

func sites() []site {
	return []site{
		{slug: "dimmi", center: helpcenter.Center{
			Name: "dimmi Help Center", Product: "dimmi", Brand: "#171717",
			LogoLight: "/assets/dimmi-logo-light.svg", LogoDark: "/assets/dimmi-logo-dark.svg",
			Tagline: "Guides and answers for teams supporting customers with dimmi.",
			HomeURL: "https://trydimmi.com", ContactURL: "mailto:support@trydimmi.com",
		}},
		{slug: "acme", center: helpcenter.Center{
			Name: "Acme Help", Product: "Acme", Brand: "#5b5bd6",
			Tagline: "Everything you need to get the most out of Acme.",
			HomeURL: "https://example.com", ContactURL: "mailto:help@example.com",
		}},
	}
}

/*
person returns the author with the key used in the content's front
matter.
*/
func person(key string) (helpcenter.Person, bool) {
	switch key {
	case "maya":
		return helpcenter.Person{Name: "Maya Kerr", Initials: "MK", Role: "Support lead"}, true
	case "jonas":
		return helpcenter.Person{Name: "Jonas Reed", Initials: "JR", Role: "Engineer"}, true
	case "ama":
		return helpcenter.Person{Name: "Ama Owusu", Initials: "AO", Role: "Product"}, true
	default:
		return helpcenter.Person{}, false
	}
}

func collections() []collection {
	return []collection{
		{
			slug: "getting-started", title: "Getting started", icon: "rocket",
			description: "Set up your workspace and answer your first conversation.",
		},
		{
			slug: "messenger", title: "Messenger", icon: "chat",
			description: "Install the messenger, match it to your brand and know who you're talking to.",
		},
		{
			slug: "messenger-install", parent: "messenger", title: "Installation", icon: "code",
			description: "Add the messenger to your site or app, whatever it's built with.",
		},
		{
			slug: "inbox", title: "Shared inbox", icon: "inbox",
			description: "Assign, snooze and close conversations as a team.",
		},
		{
			slug: "agent", title: "dimmi agent", icon: "sparkle",
			description: "Let AI answer the questions your help center already covers.",
		},
		{
			slug: "help-center", title: "Publishing", icon: "book",
			description: "Write articles in Markdown and publish them on your own domain.",
		},
		{
			slug: "integrations", title: "Integrations", icon: "plug",
			description: "Connect GitHub, Slack and the tools your team already uses.",
		},
		{
			slug: "self-hosting", title: "Self-hosting", icon: "server",
			description: "Run dimmi on your own servers with Docker.",
		},
		{
			slug: "billing", title: "Account & billing", icon: "card",
			description: "Plans, seats, invoices and canceling.",
		},
	}
}

/*
label returns the changelog label with the slug.
*/
func label(slug string) (changelog.Label, bool) {
	for _, l := range labels() {
		if l.Slug == slug {
			return l, true
		}
	}

	return changelog.Label{}, false
}

func labels() []changelog.Label {
	return []changelog.Label{
		{Name: "New", Slug: "new", Color: "#16a34a"},
		{Name: "Improved", Slug: "improved", Color: "#2563eb"},
		{Name: "Fixed", Slug: "fixed", Color: "#ea580c"},
	}
}

/*
reactions lists the emoji visitors react with, in the order the
reaction bar shows them.
*/
func reactions() []changelog.Reaction {
	return []changelog.Reaction{
		{Emoji: "👍", Name: "Thumbs up"},
		{Emoji: "❤️", Name: "Love"},
		{Emoji: "🎉", Name: "Celebrate"},
		{Emoji: "🚀", Name: "Rocket"},
		{Emoji: "👀", Name: "Eyes"},
	}
}
