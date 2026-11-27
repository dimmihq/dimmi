package helpcenter_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/app/support/helpsample"
	"github.com/dimmihq/dimmi/lang"
	"github.com/dimmihq/dimmi/resources/views/changelog"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
pageFixtures renders every page of the help center in the states that
change what it shows, from the sample help center's content.
*/
func pageFixtures(t *testing.T) map[string]view.Component {
	t.Helper()
	tr, err := i18n.New(i18n.Config{Locale: i18n.Source}, lang.Files())
	if err != nil {
		t.Fatal(err)
	}
	lib, err := helpsample.Load()
	if err != nil {
		t.Fatal(err)
	}
	theme, err := themes.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	built := view.Func(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w,
			`<link rel="stylesheet" href="/build/portal.css"><script type="module" src="/build/portal.js"></script>`)
		return err
	})
	plain, _ := lib.Center("dimmi", "/hc/dimmi")
	plain.Lang, plain.Origin = tr, "https://help.example.com"
	plain.Theming = helpcenter.Theming{Theme: theme, Assets: built, FormToken: "token"}

	rich, _ := lib.Center("acme", "")
	rich.Lang, rich.Origin, rich.Theming = tr, "https://help.acme.test", plain.Theming
	rich.Mark = "https://app.example.com/logo.png"
	rich.Messenger, rich.MessengerAI, rich.MessengerSrc = "app_1", true, "https://app.example.com/widget/v1/loader.js"
	rich.MessengerIdentity = "/messenger-identity"
	rich.FooterText = "Acme Inc.\nMade with care."
	rich.HomeTitle = "What do you need?"
	rich.Canonical = "https://help.acme.test/"
	rich.Languages = []helpcenter.LanguageLink{
		{Code: "en", Name: "English", URL: "/", Current: true}, {Code: "fr", Name: "Français", URL: "/fr"},
	}
	rich.Alternates = []helpcenter.Alternate{{Lang: "en", URL: "https://help.acme.test/"}, {Lang: "x-default", URL: "https://help.acme.test/"}}
	visitor := helpsample.Visitor()
	rich.Visitor = &visitor

	fixed := plain
	fixed.Appearance, fixed.NoIndex, fixed.SignedIn = "dark", true, true
	fixed.SignOutURL, fixed.SignInURL = "/hc/dimmi/sso/sign-out", ""
	fixed.Locale = "ar"
	fixed.Messenger, fixed.MessengerSrc = "app_2", "https://app.example.com/widget/v1/loader.js"

	all := lib.Collections()
	col, trail, _ := lib.Collection(all[0].Slug)
	article, _, err := lib.Article(plain, "install-with-a-script-tag")
	if err != nil {
		t.Fatal(err)
	}
	aged := article
	aged.Age, aged.Untranslated, aged.OriginalLang = "3 days ago", "Français", "en"
	results := lib.Search("install")

	statuses := lib.Feedback()
	ideas := statuses[0].Ideas
	ideas[0].Path = "/feedback/" + ideas[0].Slug
	boards := []helpcenter.BoardTab{{Name: "Features", Slug: "features"}, {Name: "Bugs", Slug: "bugs"}}
	index, _, err := lib.Changelog(plain, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	entry, _, err := lib.Entry(plain, index.Entries[0].Slug)
	if err != nil {
		t.Fatal(err)
	}
	convs := lib.Conversations()
	fields := []helpcenter.RequestField{
		{
			Key: "where", Label: "Where", DataType: "single_choice", Options: []string{"Web", "iOS"},
			Required: true, Value: "iOS",
		},
		{Key: "notes", Label: "Notes", DataType: "long_text", Description: "Anything else", Value: "x"},
		{
			Key: "tags", Label: "Tags", DataType: "multiple_choice", Options: []string{"a", "b"},
			ShowField: "where", ShowEquals: "iOS",
		},
		{Key: "urgent", Label: "Urgent", DataType: "boolean", Value: "true"},
		{Key: "amount", Label: "Amount", DataType: "currency"},
		{Key: "when", Label: "When", DataType: "date", Required: true},
		{Key: "other", Label: "Other", DataType: "text"},
	}
	types := []helpcenter.RequestType{
		{ID: "tt_1", Name: "Bug", Icon: "bug", Description: "Something broke"},
		{ID: "tt_2", Name: "Question", Icon: "question"},
	}
	requests := []helpcenter.Request{
		{
			Ref: "T-1", Title: "Login fails", Type: "Bug", TypeIcon: "bug", Label: "We're on it",
			Group: "in_progress", Updated: "2h", Mine: true,
		},
		{
			Ref: "T-2", Title: "Invoice", Type: "Question", TypeIcon: "question", Label: "Waiting on you",
			Group: "waiting_customer", Updated: "1d",
		},
	}
	reader := helpcenter.ReaderForm{
		Action: "/hc/dimmi/sso/codes", CheckURL: "/hc/dimmi/sso/codes/check", ResendURL: "/hc/dimmi/sso/codes",
		RestartURL: "/hc/dimmi/sso/sign-in", Next: "/hc/dimmi/tickets", Email: "ada@example.com",
		Masked: "a**@example.com",
	}
	withProduct := reader
	withProduct.ProductURL, withProduct.Error = "https://app.example.com/login", "That code is wrong."
	comments := []helpcenter.Comment{
		{
			Author: "Grace", Body: "Me too\nplease", At: "2d",
			Replies: []helpcenter.Comment{{Author: "Team", FromTeam: true, HTML: "<p>On it</p>", At: "1d"}},
		},
	}
	when := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)

	return map[string]view.Component{
		"home-plain": helpcenter.Home(plain, all, lib.Popular(), lib.Updates(3)),
		"home-rich": helpcenter.Home(
			rich, all, lib.Popular(),
			[]helpcenter.Update{{Title: "New", Path: "/changelog/new", Label: "New", Published: when}},
		),
		"home-fixed":    helpcenter.Home(fixed, all, nil, nil),
		"collection":    helpcenter.CollectionPage(plain, all, col, trail),
		"collection-ai": helpcenter.CollectionPage(rich, all, col, trail),
		"article":       helpcenter.ArticlePage(plain, all, article, helpcenter.Helpful{}),
		"article-yes":   helpcenter.ArticlePage(rich, all, aged, helpcenter.Helpful{Answer: "yes"}),
		"article-no":    helpcenter.ArticlePage(rich, all, article, helpcenter.Helpful{Answer: "no", Limited: true}),
		"article-done":  helpcenter.ArticlePage(fixed, all, aged, helpcenter.Helpful{Answer: "no", Commented: true}),
		"search-empty":  helpcenter.SearchPage(plain, all, "", nil),
		"search-none":   helpcenter.SearchPage(rich, all, "zzz", nil),
		"search-none-2": helpcenter.SearchPage(plain, all, "zzz", nil),
		"search-results": helpcenter.SearchPage(
			plain, all, "install",
			append(
				results,
				helpcenter.Result{
					Kind: "changelog", Title: "C", Path: "/changelog/c", Excerpt: "e", Trail: "Changelog",
				},
			),
		),
		"404":             helpcenter.NotFoundPage(rich, all, lib.Popular()),
		"sign-in-return":  helpcenter.ReaderSignInPage(plain, "/hc/dimmi/sso", "/hc/dimmi/tickets"),
		"sign-in":         helpcenter.ReaderEmailPage(plain, reader),
		"sign-in-product": helpcenter.ReaderEmailPage(rich, withProduct),
		"sign-in-code":    helpcenter.ReaderCodePage(plain, withProduct),
		"board": helpcenter.FeedbackPage(
			plain,
			helpcenter.Board{Statuses: statuses, Boards: boards, Active: "bugs", Submitted: true, Pending: true},
		),
		"board-list": helpcenter.FeedbackPage(
			rich, helpcenter.Board{List: ideas, Sort: "top", Query: "dark", Submitted: true},
		),
		"board-empty": helpcenter.FeedbackPage(plain, helpcenter.Board{Query: "x"}),
		"idea-new": helpcenter.NewIdeaPage(
			plain,
			helpcenter.IdeaForm{
				Boards: boards, Board: "bugs", Title: "T", Details: "D", Similar: ideas, Error: "Too short",
			},
		),
		"idea-new-one": helpcenter.NewIdeaPage(rich, helpcenter.IdeaForm{Board: "bugs"}),
		"idea": helpcenter.IdeaPage(
			plain,
			helpcenter.IdeaView{
				Idea: ideas[0], Status: statuses[2], Details: "<p>More</p>", Author: "Ada", Posted: "3d",
				Comments: comments, Update: "/changelog/x", Pending: true, CanAct: true, Commented: true,
			},
		),
		"idea-sign-in": helpcenter.IdeaPage(
			rich,
			helpcenter.IdeaView{Idea: ideas[1], Status: statuses[0], Author: "Ada", Posted: "3d", SignIn: "/sign-in"},
		),
		"changelog": changelog.IndexPage(plain, index),
		"changelog-sub": changelog.IndexPage(
			rich,
			changelog.Index{
				Title: "Updates", Entries: index.Entries, Labels: index.Labels, Active: index.Labels[0].Slug,
				Newer: "/changelog", Older: "/changelog/page/3", Subscribed: true,
			},
		),
		"changelog-none": changelog.IndexPage(plain, changelog.Index{NoSubscribe: true}),
		"entry":          changelog.EntryPage(plain, index.Labels, entry),
		"entry-rich": changelog.EntryPage(
			rich, index.Labels,
			changelog.Page{Entry: entry.Entry, Previous: &index.Entries[1], Title: "Updates", NoSubscribe: true},
		),
		"subscription": changelog.SubscriptionPage(
			plain, changelog.Subscription{Purpose: "confirm", Email: "a@b.c", Action: "/x"},
		),
		"unsubscribed": changelog.SubscriptionPage(
			plain, changelog.Subscription{Purpose: "unsubscribe", Email: "a@b.c", Done: true},
		),
		"sub-invalid": changelog.SubscriptionPage(
			rich, changelog.Subscription{Purpose: "unsubscribe", Invalid: true},
		),
		"tickets": helpcenter.RequestsPage(
			plain, helpcenter.Requests{Items: requests, Types: types, Company: true, Scope: "company"},
		),
		"tickets-empty": helpcenter.RequestsPage(rich, helpcenter.Requests{}),
		"ticket-new":    helpcenter.NewRequestPage(plain, helpcenter.RequestForm{Types: types}),
		"ticket-form": helpcenter.NewRequestPage(
			plain,
			helpcenter.RequestForm{
				Type: types[0], Types: types, Fields: fields, Title: "T", Description: "D", Error: "Fill it",
				Key: "k1",
			},
		),
		"ticket": helpcenter.RequestShowPage(
			plain,
			helpcenter.RequestPage{
				Request: requests[0], Steps: helpcenter.Progress("in_progress"),
				Fields:   []helpcenter.RequestField{{Label: "Where", Value: "iOS"}},
				Messages: convs[0].Messages, Error: "Too long",
			},
		),
		"ticket-plain": helpcenter.RequestShowPage(
			rich, helpcenter.RequestPage{Request: requests[1], Steps: helpcenter.Progress("resolved")},
		),
		"demo-sign-in":    helpcenter.SignInPage(plain, all, false),
		"demo-convs":      helpcenter.ConversationsPage(rich, convs),
		"demo-conv":       helpcenter.ConversationPage(plain, convs, convs[0]),
		"demo-conv-new":   helpcenter.NewConversationPage(plain, convs),
		"demo-conv-new-2": helpcenter.NewConversationPage(rich, convs),
	}
}

/*
TestPagesRender renders every page of the help center in the default
theme, in the states that change what it shows.
*/
func TestPagesRender(t *testing.T) {
	t.Parallel()
	for name, page := range pageFixtures(t) {
		ctx := view.WithNonce(context.Background(), "NONCE")
		html, err := view.HTML(ctx, page)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(html, "<html") || !strings.Contains(html, `nonce="NONCE"`) {
			t.Errorf("%s did not render a whole page:\n%s", name, html)
		}
	}
}
