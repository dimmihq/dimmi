/*
|--------------------------------------------------------------------------
| Theme Variables: The Help Center
|--------------------------------------------------------------------------
|
| What every page reads about the help center, the reader and the
| page's language, shaped from the view models: links are full paths
| under the help center's base and words are in the page's language.
| The fields' names in snake case are the variables' names, such as
| help_center.search_url, and their doc tags are what the theme
| reference says about them.
|
*/

package helpcenter

/*
CenterVars is help_center, on every page.
*/
type CenterVars struct {
	// The help center's name.
	Name string
	// The product it supports, shown beside the logo.
	Product string
	// The line under the home page's greeting, empty for none.
	Tagline string
	// The question the home page asks, the team's own or the default.
	HomeTitle string
	// The team's text at the foot of every page.
	FooterText string
	// The brand color as a hex code, such as #10b981.
	Brand string
	// A style declaring --brand, for the html element.
	BrandStyle string
	// The logo's address for light pages, empty for none.
	LogoLight string
	// The logo's address for dark pages.
	LogoDark string
	// The company's square logo's address, empty for none.
	Mark string
	// The picture the browser tab shows, empty for none.
	TabIcon string
	// Light or dark when the team fixed the theme, empty when readers pick.
	Appearance string
	// The color-scheme meta tag's content.
	ColorScheme string
	// The help center shows its messenger.
	Messenger bool
	// The AI agent answers the messenger first.
	AskAI bool
	// Ask AI or Contact us, the label of the button that opens the messenger.
	HelpLabel string
	// The icon of that button.
	HelpIcon string
	// A reader is signed in through the team's product.
	SignedIn bool
	// Where a reader signs in, empty when the help center takes no sign-in.
	SignInURL string
	// Where a reader signed in through the team's product signs out.
	SignOutURL string
	// The team's contact address, empty for none.
	ContactURL string
	// The help center's home page.
	HomeURL string
	// The search page; search forms send q to it.
	SearchURL string
	// The changelog.
	ChangelogURL string
	// The feature requests page.
	FeedbackURL string
	// Where the changelog's subscribe form posts.
	SubscribeURL string
	// The signed-in reader's requests.
	TicketsURL string
	// The portal's sections in navigation order, the help center first.
	Sections []SectionVars
	// The scheme and host the page is served on.
	Origin string
	// The path every link starts with, empty on the help center's own domain.
	Base string

	// Sample help center only: the reader's conversations.
	ConversationsURL string `liquid:"conversations_url"`
	// Sample help center only: a new conversation.
	NewConversationURL string
	// Sample help center only: where its sign-in form posts.
	DemoSignInURL string
	// Sample help center only: where its sign-out form posts.
	DemoSignOutURL string
}

/*
SectionVars is a part of the portal, such as the changelog.
*/
type SectionVars struct {
	// Help, changelog, feedback or requests; page.section names the current one.
	Key string
	// Its name in the page's language.
	Name string
	// Its icon's name, for the icon filter.
	Icon string
	// Its address.
	URL string
}

/*
PersonVars is a person: a reader, an author or a teammate.
*/
type PersonVars struct {
	// Their name.
	Name string
	// Their initials, drawn when there is no photo.
	Initials string
	// Their role, empty for none.
	Role string
	// Their photo's address, empty for none.
	Photo string
	// For reader only: their conversations.
	ConversationsURL string
}

/*
LocaleVars is locale, the page's language and the others it is in.
*/
type LocaleVars struct {
	// The page's language tag, such as en or pt-BR.
	Code string
	// Ltr or rtl.
	Dir string
	// The page's language named in itself.
	Name string
	// The languages a reader may switch to, none when the help center speaks one.
	Languages []LanguageVars
	// This page's addresses in other languages, for search engines.
	Alternates []AlternateVars
}

/*
LanguageVars is a language in the language menu.
*/
type LanguageVars struct {
	// Its tag.
	Code string
	// Its name in itself.
	Name string
	// This page in it, or its home when the page is not translated.
	URL string
	// It is the page's language.
	Current bool
}

func (c Center) url(path string) string { return Href(c, path) }

func (c Center) helpCenterVars() CenterVars {
	v := CenterVars{
		Name: c.Name, Product: c.Product, Tagline: c.Tagline, FooterText: c.FooterText, Brand: c.Brand,
		BrandStyle: BrandStyle(c), Appearance: fixedTheme(c), ColorScheme: colorScheme(c),
		Messenger: c.Messenger != "", AskAI: c.MessengerAI, HelpLabel: HelpLabel(c), HelpIcon: "chat",
		SignedIn: c.SignedIn, SignInURL: c.SignInURL, SignOutURL: c.SignOutURL, ContactURL: c.ContactURL,
		HomeURL: c.url("/"), SearchURL: c.url("/search"), ChangelogURL: c.url("/changelog"),
		FeedbackURL: c.url("/feedback"), SubscribeURL: c.url("/changelog/subscriptions"), TicketsURL: c.url("/tickets"),
		Origin: c.Origin, Base: c.Base,
		ConversationsURL: c.url("/conversations"), NewConversationURL: c.url("/conversations/new"),
		DemoSignInURL: c.url("/sign-in"), DemoSignOutURL: c.url("/sign-out"),
		HomeTitle: c.HomeTitle,
	}
	if v.HomeTitle == "" {
		v.HomeTitle = c.T("home_title")
	}
	if c.MessengerAI {
		v.HelpIcon = "sparkle"
	}
	if c.LogoLight != "" {
		v.LogoLight, v.LogoDark = Asset(c, c.LogoLight), Asset(c, c.LogoDark)
	}
	if c.Mark != "" {
		v.Mark = Asset(c, c.Mark)
	}
	switch {
	case c.Mark != "":
		v.TabIcon = Asset(c, c.Mark)
	case c.LogoLight != "":
		v.TabIcon = Asset(c, c.LogoLight)
	}
	for _, s := range Sections(c) {
		v.Sections = append(v.Sections, SectionVars{Key: s.Key, Name: s.Name, Icon: s.Icon, URL: c.url(s.Path)})
	}
	return v
}

func (c Center) localeVars() LocaleVars {
	v := LocaleVars{Code: pageLang(c), Dir: pageDir(c), Name: pageLang(c)}
	for _, l := range c.Languages {
		v.Languages = append(v.Languages, LanguageVars(l))
		if l.Current {
			v.Name = l.Name
		}
	}
	for _, a := range c.Alternates {
		v.Alternates = append(v.Alternates, AlternateVars(a))
	}
	return v
}

func (c Center) readerVars() *PersonVars {
	if c.Visitor == nil {
		return nil
	}
	p := personVars(*c.Visitor)
	p.ConversationsURL = c.url("/conversations")
	return &p
}

func personVars(p Person) PersonVars {
	return PersonVars{Name: p.Name, Initials: p.Initials, Role: p.Role, Photo: p.Photo}
}

func people(list []Person) []PersonVars {
	out := make([]PersonVars, 0, len(list))
	for _, p := range list {
		out = append(out, personVars(p))
	}
	return out
}
