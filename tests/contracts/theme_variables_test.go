/*
|--------------------------------------------------------------------------
| The Theme Variables Reference
|--------------------------------------------------------------------------
|
| Help center themes read variables shaped by the view models in
| resources/views/helpcenter and resources/views/changelog. The docs page
| that lists them is written from those structs and their field
| comments, so a variable added to a page shows up in the reference, and
| a page's variables never drift from what templates receive. Refresh it
| with make api-snapshot.
|
*/

package contracts_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/liquid"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
liquidTag reads the name a struct field's liquid tag gives it.
*/
var liquidTag = regexp.MustCompile(`liquid:"([^"]+)"`)

/*
themeVariable is a variable a page reads: its name, the type behind it
or a fixed kind, and what it holds.
*/
type themeVariable struct {
	name, typ, about string
}

/*
everyPage are the variables every template, partial and layout reads.
*/
func everyPage() []themeVariable {
	return []themeVariable{
		{"help_center", "CenterVars", "The help center the page belongs to."},
		{"reader", "PersonVars", "The signed-in reader of the sample help center, nil otherwise."},
		{"locale", "LocaleVars", "The page's language and the others it is in."},
		{"page", "PageVars", "What the page says about itself."},
		{"settings", "map", "The theme's settings by id: the team's values, else the defaults in theme.json."},
		{"translations", "map", "Every word of the help center in the page's language, by key."},
		{
			"platform_assets", "HTML",
			"dimmi's stylesheet and script and the theme switch. Keep it to keep the default look.",
		},
		{"seo_tags", "HTML", "Description, preview, robots, canonical, hreflang, Markdown and feed tags."},
		{"messenger_snippet", "HTML", "The script that loads the help center's messenger, empty when it shows none."},
		{"form_token", "text", "The anti-forgery token every form that posts must send as _token."},
		{"form_token_field", "HTML", "The hidden field holding form_token."},
		{"script_nonce", "text", "The nonce an inline script needs on the help center's own domain, empty elsewhere."},
		{"content_for_layout", "HTML", "In layouts only: the page's own output."},
	}
}

/*
pageVariables are the variables each page template reads besides the
ones every page has.
*/
func pageVariables() []struct {
	pages string
	vars  []themeVariable
} {
	collections := themeVariable{"collections", "[]CollectionVars", "Every top-level collection, for the drawer."}
	breadcrumb := themeVariable{"breadcrumb", "[]CrumbVars", "The way back from the page, home first."}
	return []struct {
		pages string
		vars  []themeVariable
	}{
		{"home", []themeVariable{
			collections,
			{"popular", "[]SummaryVars", "The articles the home page suggests."},
			{"updates", "[]UpdateVars", "The newest changelog entries."},
		}},
		{"collection", []themeVariable{
			collections, breadcrumb,
			{"collection", "CollectionVars", "The collection."},
			{"in_collection", "CollectionVars", "Its sub-collections and articles as one group to list."},
		}},
		{"article", []themeVariable{collections, breadcrumb, {"article", "ArticleVars", "The article."}}},
		{"search", []themeVariable{collections, {"search", "SearchVars", "The question and what it found."}}},
		{"404", []themeVariable{collections, {"popular", "[]SummaryVars", "Articles to suggest."}}},
		{"changelog", []themeVariable{{"changelog", "LogVars", "The changelog's page of entries."}}},
		{"changelog_entry", []themeVariable{
			{"entry", "EntryVars", "The entry."},
			{
				"previous", "EntryVars", "The older entry, nil for none.",
			},
			{"next", "EntryVars", "The newer entry, nil for none."},
			{"changelog", "LogVars", "Its title and whether it takes subscriptions."},
		}},
		{
			"changelog_subscription",
			[]themeVariable{{"subscription", "SubscriptionVars", "The link's purpose and state."}},
		},
		{"feature_requests", []themeVariable{{"board", "BoardVars", "The board."}}},
		{"feature_request", []themeVariable{{"request", "RequestViewVars", "The request and its comments."}}},
		{"feature_request_new", []themeVariable{{"form", "IdeaFormVars", "The form, filled in again when needed."}}},
		{"sign_in, sign_in_code, sign_in_return", []themeVariable{{"sign_in", "ReaderForm", "The sign-in forms."}}},
		{"tickets", []themeVariable{{"tickets", "TicketsVars", "The reader's requests."}}},
		{"ticket", []themeVariable{{"ticket", "TicketVars", "The request with its thread."}}},
		{"ticket_new", []themeVariable{{"ticket_form", "TicketFormVars", "The form for the type picked."}}},
	}
}

func TestThemeVariablesContract(t *testing.T) {
	t.Parallel()

	types := map[string]*ast.StructType{}
	docs := map[string]string{}
	for _, dir := range []string{"../../resources/views/helpcenter", "../../resources/views/changelog"} {
		readStructs(t, dir, types, docs)
	}

	var b strings.Builder
	b.WriteString(themeVariablesIntro)
	b.WriteString("\n## Every page\n\n| Variable | Type | What it holds |\n| --- | --- | --- |\n")
	for _, v := range everyPage() {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", v.name, typeLink(v.typ), v.about)
	}
	b.WriteString("\n## Each page\n")
	for _, p := range pageVariables() {
		fmt.Fprintf(&b, "\n### %s\n\n| Variable | Type | What it holds |\n| --- | --- | --- |\n", pageNames(p.pages))
		for _, v := range p.vars {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", v.name, typeLink(v.typ), v.about)
		}
	}
	b.WriteString("\n## Objects\n")
	names := make([]string, 0, len(types))
	for name := range types {
		if strings.HasSuffix(name, "Vars") || name == "ReaderForm" || name == "RequestField" || name == "RequestStep" ||
			name == "BoardTab" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		writeObject(&b, object{name: name, st: types[name], doc: docs[name], types: types})
	}
	testkit.Golden(t, "../../docs/src/content/docs/help-center/theme-variables.mdx", []byte(b.String()))
	require.NotEmpty(t, names)
}

const themeVariablesIntro = `---
title: "Theme variables"
description: "Every variable a help center theme's templates read, page by page, written from dimmi's code."
---

{/* Written by tests/contracts/theme_variables_test.go. Run make api-snapshot after changing a view model. */}

Every template gets the variables under **Every page**, and its own under **Each page**. Partials see the same
globals, plus the arguments passed to ` + "`{% render %}`" + `. Links are full paths under the help center's base, and
words are already in the page's language.

Print a variable with ` + "`{{ article.title }}`" + `. Text is escaped for HTML; values marked HTML print as markup.
`

func pageNames(pages string) string {
	parts := strings.Split(pages, ", ")
	for i, p := range parts {
		parts[i] = "`" + p + "`"
	}
	return strings.Join(parts, ", ")
}

/*
typeLink names a variable's type, linking an object to its section.
*/
func typeLink(typ string) string {
	switch {
	case typ == "map", typ == "HTML", typ == "text":
		return typ
	case strings.HasPrefix(typ, "[]"):
		return "list of " + typeLink(typ[2:])
	}
	return "[" + objectName(typ) + "](#" + strings.ToLower(objectName(typ)) + ")"
}

/*
objectName is how the reference names an object: its Go type without
the Vars ending.
*/
func objectName(typ string) string {
	name := strings.TrimSuffix(typ, "Vars")
	if name == "" {
		return typ
	}
	return name
}

func readStructs(t *testing.T, dir string, types map[string]*ast.StructType, docs map[string]string) {
	t.Helper()
	fset := token.NewFileSet()
	notTests := func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }
	// The views carry no build tags, so reading the folder whole is exact.
	pkgs, err := parser.ParseDir(fset, dir, notTests, parser.ParseComments) //nolint:staticcheck // see above
	require.NoError(t, err)
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				readDecl(decl, types, docs)
			}
		}
	}
}

/*
readDecl keeps the struct types a declaration names, with their doc.
*/
func readDecl(decl ast.Decl, types map[string]*ast.StructType, docs map[string]string) {
	gen, ok := decl.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, spec := range gen.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			types[ts.Name.Name] = st
			docs[ts.Name.Name] = strings.TrimSpace(gen.Doc.Text())
		}
	}
}

/*
object is one struct the reference lists, with every struct it may name.
*/
type object struct {
	name  string
	st    *ast.StructType
	doc   string
	types map[string]*ast.StructType
}

func writeObject(b *strings.Builder, o object) {
	name, st, doc, types := o.name, o.st, o.doc, o.types
	fmt.Fprintf(b, "\n### %s\n\n", objectName(name))
	if doc != "" {
		doc = strings.Join(strings.Fields(doc), " ")
		doc = strings.Replace(doc, name+" is ", objectName(name)+" is ", 1)
		fmt.Fprintf(b, "%s\n\n", doc)
	}
	b.WriteString("| Field | Type | What it holds |\n| --- | --- | --- |\n")
	for _, row := range fieldRows(st, types) {
		b.WriteString(row)
	}
}

func fieldRows(st *ast.StructType, types map[string]*ast.StructType) []string {
	var rows []string
	for _, f := range st.Fields.List {
		typ := exprType(f.Type)
		if len(f.Names) == 0 {
			if embedded, ok := types[typ]; ok {
				rows = append(rows, fieldRows(embedded, types)...)
			}
			continue
		}
		name := liquid.SnakeCase(f.Names[0].Name)
		if f.Tag != nil {
			if m := liquidTag.FindStringSubmatch(f.Tag.Value); m != nil {
				name = m[1]
			}
		}
		about := strings.Join(strings.Fields(f.Doc.Text()), " ")
		rows = append(rows, fmt.Sprintf("| %#q | %s | %s |\n", name, fieldType(typ, types), about))
	}
	return rows
}

func exprType(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return exprType(x.X)
	case *ast.ArrayType:
		return "[]" + exprType(x.Elt)
	case *ast.SelectorExpr:
		return exprType(x.X) + "." + x.Sel.Name
	case *ast.MapType:
		return "map"
	}
	return "value"
}

func fieldType(typ string, types map[string]*ast.StructType) string {
	if strings.HasPrefix(typ, "[]") {
		return "list of " + fieldType(typ[2:], types)
	}
	base := typ
	if i := strings.LastIndexByte(typ, '.'); i >= 0 {
		base = typ[i+1:]
	}
	switch typ {
	case "string":
		return "text"
	case "bool":
		return "true or false"
	case "int", "int32", "int64", "float64":
		return "number"
	case "time.Time":
		return "time"
	case "liquid.HTML":
		return "HTML"
	case "map":
		return "map"
	}
	if _, ok := types[base]; ok {
		return "[" + objectName(base) + "](#" + strings.ToLower(objectName(base)) + ")"
	}
	return base
}
