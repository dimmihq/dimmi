package intercom

import (
	"encoding/json"
	"html"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/dimmihq/dimmi/framework/dom"

	"github.com/dimmihq/dimmi/cli/format"
)

func translateHelpCenter(h helpCenter, defaultLocale string) format.Record {
	locales := slices.Clone(h.Locales)
	if defaultLocale == "" && len(locales) > 0 {
		defaultLocale = locales[0]
	}

	return format.NewRecord(format.KindHelpCenter, string(h.ID), &format.HelpCenter{
		Base: base(h.ID, h.UpdatedAt), Name: text(firstOf(h.DisplayName, h.Identifier)), DefaultLocale: defaultLocale,
		Locales: locales, URL: h.URL,
	})
}

/*
translatedGroup reads a collection's translated_content: a map from
locale to name and description, with a "type" key among them.
*/
func translatedGroup(raw map[string]json.RawMessage) map[string]groupContent {
	out := map[string]groupContent{}

	for locale, v := range raw {
		if locale == "type" || string(v) == "null" {
			continue
		}

		var g groupContent
		if json.Unmarshal(v, &g) == nil && (g.Name != "" || g.Description != "") {
			out[locale] = g
		}
	}

	return out
}

func translateCollection(c collection, helpCenter string) format.Record {
	col := &format.Collection{
		Base: base(c.ID, c.UpdatedAt), HelpCenterRef: helpCenter, ParentRef: string(c.ParentID),
		Name: text(c.Name), Description: text(c.Description), Icon: c.Icon, Order: c.Order,
	}

	tr := translatedGroup(c.Translated)
	for _, locale := range sortedKeys(tr) {
		if locale == c.DefaultLocale {
			continue
		}

		g := tr[locale]
		col.Translations = append(col.Translations, format.Translation{
			Locale: locale, Name: text(g.Name), Description: text(g.Description),
		})
	}

	return format.NewRecord(format.KindCollection, col.SourceID, col)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

/*
articleState maps Intercom's published and draft.
*/
func articleState(s string) string {
	if s == "published" {
		return "published"
	}

	return "draft"
}

func teammateAuthor(id ID) *format.Author {
	if id == "" || id == "0" {
		return nil
	}

	return &format.Author{Type: format.AuthorTeammate, Ref: string(id)}
}

/*
articleBody converts an article's HTML body, which is faithful to what
Intercom shows; body_markdown drops blocks such as buttons and videos.
*/
func articleBody(body, bodyMarkdown string) string {
	if strings.TrimSpace(body) == "" {
		return strings.TrimSpace(bodyMarkdown)
	}

	md, err := dom.ToMarkdown(body)
	if err != nil {
		if bodyMarkdown != "" {
			return strings.TrimSpace(bodyMarkdown)
		}

		return strings.TrimSpace(dom.ToText(body))
	}

	return strings.TrimSpace(md)
}

/*
block is a rich block an Intercom article body uses, found by the
markup that marks it.
*/
type block struct {
	name  string
	marks []string
}

/*
in reports whether the body uses the block.
*/
func (b block) in(body string) bool {
	return slices.ContainsFunc(b.marks, func(m string) bool { return strings.Contains(body, m) })
}

/*
blocks lists the rich blocks by name.
*/
func blocks() []block {
	return []block{
		{"button", []string{"intercom-h2b-button"}},
		{"callout", []string{"intercom-interblocks-callout"}},
		{"centered", []string{"intercom-align-center"}},
		{"code", []string{"<pre"}},
		{"collapsible", []string{"<details", "intercom-interblocks-collapsible"}},
		{"horizontal_rule", []string{"<hr"}},
		{"image", []string{"<img"}},
		{"table", []string{"<table"}},
		{"video", []string{"<iframe"}},
	}
}

func blocksOf(bodies ...string) []string {
	var out []string

	for _, b := range blocks() {
		if slices.ContainsFunc(bodies, b.in) {
			out = append(out, b.name)
		}
	}

	return out
}

/*
audience maps help_center_audience; anything narrower than everyone is
kept by name for dimmi's audiences.
*/
func audience(a string) string {
	switch a {
	case "", "everyone":
		return ""
	}

	return a
}

/*
redirect is the path of an article's old address, which dimmi's help
center redirects from.
*/
func redirect(u string) string {
	if u == "" {
		return ""
	}

	if _, after, ok := strings.Cut(u, "://"); ok {
		rest := after
		if j := strings.Index(rest, "/"); j >= 0 {
			return rest[j:]
		}

		return ""
	}

	return u
}

/*
images collects the pictures article bodies show. Each is uploaded as a
file and the body points at it as dimmi-file:<id>, which the engine
turns into the uploaded file's id and dimmi copies into the help
center, so nothing links back to Intercom's CDN.
*/
type images struct {
	files []*format.File
	seen  map[string]bool
}

var imgSrc = regexp.MustCompile(`(<img\b[^>]*?\ssrc=")([^"]+)(")`)

/*
rewrite points every picture of the HTML at its upload.
*/
func (im *images) rewrite(body string) string {
	return imgSrc.ReplaceAllStringFunc(body, func(m string) string {
		parts := imgSrc.FindStringSubmatch(m)
		src := html.UnescapeString(parts[2])

		if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
			return m
		}

		name := path.Base(strings.SplitN(src, "?", 2)[0])
		id := format.FileID(src, name, 0)

		if im.seen == nil {
			im.seen = map[string]bool{}
		}

		if !im.seen[id] {
			im.seen[id] = true
			im.files = append(im.files, &format.File{SourceID: id, Name: name, Source: src})
		}

		return parts[1] + "dimmi-file:" + id + parts[3]
	})
}

func translateArticle(a article, helpCenter, defaultLocale string) format.Record {
	im := &images{}
	a.Body = im.rewrite(a.Body)

	locale := firstOf(a.DefaultLocale, defaultLocale, "en")

	art := &format.Article{
		Base: base(a.ID, a.UpdatedAt), HelpCenterRef: helpCenter, Title: text(a.Title),
		Description: text(a.Description), Body: articleBody(a.Body, a.BodyMarkdown), State: articleState(a.State),
		Visibility: format.VisibilityPublic, Audience: audience(a.Audience), Author: teammateAuthor(a.AuthorID),
		CreatedAt: when(a.CreatedAt), UpdatedAt: when(a.UpdatedAt), CollectionRef: collectionRef(a),
	}

	if r := redirect(a.URL); r != "" {
		art.Redirects = append(art.Redirects, r)
	}

	translations, bodies := articleTranslations(a, locale, im)
	art.Translations = translations
	art.Blocks = blocksOf(append([]string{a.Body}, bodies...)...)

	for _, t := range a.Tags.Tags {
		art.TagRefs = append(art.TagRefs, string(t.ID))
	}

	return format.NewRecord(format.KindArticle, art.SourceID, art, im.files...)
}

/*
collectionRef is the collection an article sits in, if its parent is
one.
*/
func collectionRef(a article) string {
	if a.ParentType != "collection" && a.ParentType != "" {
		return ""
	}

	if a.ParentID != "" {
		return string(a.ParentID)
	}

	if len(a.ParentIDs) > 0 {
		return string(a.ParentIDs[0])
	}

	return ""
}

/*
articleTranslations turns an article's translations other than its own
locale. It also returns their bodies as Intercom sent them, for the
blocks they use.
*/
func articleTranslations(a article, locale string, im *images) ([]format.Translation, []string) {
	var out []format.Translation
	var bodies []string

	for _, l := range sortedKeys(a.Translated) {
		if l == "type" || l == locale || string(a.Translated[l]) == "null" {
			continue
		}

		var c articleContent
		if json.Unmarshal(a.Translated[l], &c) != nil || (c.Title == "" && c.Body == "") {
			continue
		}

		bodies = append(bodies, c.Body)
		c.Body = im.rewrite(c.Body)
		tr := format.Translation{
			Locale: l, Title: text(c.Title), Description: text(c.Description),
			Body: articleBody(c.Body, c.BodyMarkdown), State: articleState(c.State),
			Author: teammateAuthor(c.AuthorID), CreatedAt: when(c.CreatedAt), UpdatedAt: when(c.UpdatedAt),
		}

		if r := redirect(c.URL); r != "" {
			tr.Redirects = []string{r}
		}

		out = append(out, tr)
	}

	return out, bodies
}

/*
internalID keeps internal articles apart from public ones, whose ids
Intercom draws from another sequence.
*/
func internalID(id ID) string { return "internal_" + string(id) }

func translateInternalArticle(a internalArticle, helpCenter string) format.Record {
	id := internalID(a.ID)
	im := &images{}
	a.Body = im.rewrite(a.Body)

	return format.NewRecord(format.KindArticle, id, &format.Article{
		Base:          format.Base{SourceID: id, SourceUpdatedAt: when(a.UpdatedAt)},
		HelpCenterRef: helpCenter, Title: text(a.Title),
		Body: articleBody(a.Body, ""), State: "published", Visibility: format.VisibilityAgentOnly,
		Author:    teammateAuthor(firstID(a.AuthorID, a.OwnerID)),
		CreatedAt: when(a.CreatedAt), UpdatedAt: when(a.UpdatedAt), Blocks: blocksOf(a.Body),
	}, im.files...)
}

func firstID(ids ...ID) ID {
	for _, id := range ids {
		if id != "" && id != "0" {
			return id
		}
	}

	return ""
}

func translateSnippet(s snippet, helpCenter string) format.Record {
	return format.NewRecord(format.KindSnippet, string(s.ID), &format.Snippet{
		Base: base(s.ID, s.UpdatedAt), HelpCenterRef: helpCenter, Title: text(s.Title),
		Body: strings.TrimSpace(s.BodyMarkdown), Locale: s.Locale, CreatedAt: when(s.CreatedAt),
	})
}
