/*
|--------------------------------------------------------------------------
| Themes
|--------------------------------------------------------------------------
|
| A theme is a folder: theme.json, a layout every page sits in, one
| template per page, partials the templates draw, and assets such as
| CSS, scripts, images and fonts. Loading a theme checks every file and
| parses every template, and lists every problem it finds with its file
| and line, so a theme goes live whole or not at all.
|
| A team's theme falls back to the default theme for any template,
| partial or asset it lacks. A theme written before a page existed still
| shows that page, and a small theme may change only its stylesheet.
|
*/

package themes

import (
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/liquid"
)

/*
Names of a theme's parts: its manifest, the folders of its layouts,
page templates, partials and assets, and the layout pages use unless
they pick another.
*/
const (
	ManifestFile  = "theme.json"
	LayoutDir     = "layouts/"
	TemplateDir   = "templates/"
	PartialDir    = "partials/"
	AssetDir      = "assets/"
	DefaultLayout = "theme"
)

/*
Limits on what a theme may hold.
*/
const (
	MaxFiles     = 1000
	MaxFileBytes = 5 << 20
	MaxBytes     = 25 << 20
)

/*
Problem is one thing wrong with a theme, at its file and line when it
has them.
*/
type Problem struct {
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

func (p Problem) String() string {
	switch {
	case p.File == "":
		return p.Message
	case p.Line == 0:
		return p.File + ": " + p.Message
	}
	return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Message)
}

/*
Asset is a file a theme serves, with the hash its address carries.
*/
type Asset struct {
	Path string
	Type string
	Hash string
	Body []byte
}

/*
Theme is a loaded theme: its manifest, its parsed templates and its
assets, the hash of all its files, and the theme it falls back to, nil
for the default theme itself.
*/
type Theme struct {
	Manifest Manifest
	Hash     string
	set      *liquid.Set
	assets   map[string]Asset
	fallback *Theme
}

/*
Custom says the theme is a team's own rather than the default.
*/
func (t *Theme) Custom() bool { return t.fallback != nil }

/*
Asset returns the asset at the path under assets/, the default theme's
when the theme lacks it, with the theme that holds it.
*/
func (t *Theme) Asset(name string) (Asset, *Theme, bool) {
	for at := t; at != nil; at = at.fallback {
		if a, ok := at.assets[name]; ok {
			return a, at, true
		}
	}
	return Asset{}, nil, false
}

/*
HasTemplate says whether the theme itself, not the default, holds the
template.
*/
func (t *Theme) HasTemplate(name string) bool { return t.set.Has(name) }

//go:embed all:default
var defaultFiles embed.FS

/*
DefaultFiles returns the default theme's files by path, the starting
point of a team's own theme.
*/
func DefaultFiles() (map[string][]byte, error) {
	root, err := fs.Sub(defaultFiles, "default")
	if err != nil {
		return nil, fmt.Errorf("default theme: %w", err)
	}
	files := map[string][]byte{}
	err = fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(root, p)
		files[p] = body
		return err
	})
	return files, err
}

/*
LoadDefault parses the default theme, which every help center wears
until its team publishes its own and every theme falls back to. Parse
it once and keep it.
*/
func LoadDefault() (*Theme, error) {
	files, err := DefaultFiles()
	if err != nil {
		return nil, err
	}
	t, problems := parseTheme(files, nil)
	if len(problems) > 0 {
		return nil, fmt.Errorf("default theme: %s", problems[0])
	}
	return t, nil
}

/*
Load checks a team's theme and parses it, falling back to the default
theme for what it lacks. It returns every problem it found; a theme with
any is not usable.
*/
func Load(files map[string][]byte, fallback *Theme) (*Theme, []Problem) {
	if fallback == nil {
		return nil, []Problem{{Message: "the default theme is not loaded"}}
	}
	return parseTheme(files, fallback)
}

/*
parts is what reading a theme's files found: its templates' sources,
its assets, the hash of all its files and the problems with them.
*/
type parts struct {
	sources  map[string]string
	assets   map[string]Asset
	hash     string
	problems []Problem
}

func parseTheme(files map[string][]byte, fallback *Theme) (*Theme, []Problem) {
	if len(files) > MaxFiles {
		return nil, []Problem{{Message: fmt.Sprintf("a theme holds at most %d files", MaxFiles)}}
	}
	manifest, found := files[ManifestFile]
	if !found {
		return nil, []Problem{{File: ManifestFile, Message: "theme.json is missing"}}
	}
	m, problems := parseManifest(manifest)
	p := read(files)
	problems = append(problems, p.problems...)

	var fallbackSet *liquid.Set
	if fallback != nil {
		fallbackSet = fallback.set
	}
	set, errs := newEngine().ParseSet(p.sources, partialPath, fallbackSet)
	for _, err := range errs {
		problems = append(problems, problemOf(err))
	}
	if _, ok := set.Lookup(LayoutDir + DefaultLayout); !ok {
		layout := LayoutDir + DefaultLayout + ".liquid"
		problems = append(problems, Problem{File: layout, Message: "the layout is missing"})
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return &Theme{Manifest: m, set: set, assets: p.assets, Hash: p.hash, fallback: fallback}, nil
}

/*
read sorts a theme's files into templates and assets, checking each
path and size, and hashes them all.
*/
func read(files map[string][]byte) parts {
	p := parts{sources: map[string]string{}, assets: map[string]Asset{}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	sum, total := hashing.NewSHA256(), 0
	for _, name := range names {
		body := files[name]
		total += len(body)
		_, _ = sum.Write([]byte(name + "\x00" + fmt.Sprint(len(body)) + "\x00"))
		_, _ = sum.Write(body)
		if problem := checkPath(name, len(body)); problem != "" {
			p.problems = append(p.problems, Problem{File: name, Message: problem})
			continue
		}
		p.add(name, body)
	}
	if total > MaxBytes {
		p.problems = append(p.problems, Problem{Message: fmt.Sprintf("a theme holds at most %d MB", MaxBytes>>20)})
	}
	p.hash = hex.EncodeToString(sum.Sum(nil))[:16]
	return p
}

/*
add files one checked file as an asset or a template.
*/
func (p *parts) add(name string, body []byte) {
	switch {
	case name == ManifestFile || ignored(name):
	case strings.HasPrefix(name, AssetDir):
		kind, _ := assetType(name)
		rel := strings.TrimPrefix(name, AssetDir)
		p.assets[rel] = Asset{Path: rel, Type: kind, Hash: hashing.SHA256Hex(body)[:12], Body: body}
	case !utf8.Valid(body):
		p.problems = append(p.problems, Problem{File: name, Message: "is not UTF-8 text"})
	default:
		p.sources[strings.TrimSuffix(name, ".liquid")] = string(body)
	}
}

func partialPath(name string) string { return PartialDir + name }

func problemOf(err error) Problem {
	var le *liquid.Error
	if !errors.As(err, &le) {
		return Problem{Message: err.Error()}
	}
	return Problem{File: le.Template + ".liquid", Line: le.Line, Message: le.Message}
}
