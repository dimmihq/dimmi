/*
|--------------------------------------------------------------------------
| Prompt Loader
|--------------------------------------------------------------------------
|
| The prompt tree is compiled in: resources/prompts/agent/<version>/,
| a common folder and one folder per model family. A file under the
| model's family wins over the same file in common, so a family gets
| its own tuning where the shared text does not fit.
|
| Every version is read and checked when the application boots: each
| required file exists in common, every placeholder a file names is
| one the pipeline fills, and the default lines parse. A broken prompt
| tree stops the boot instead of surfacing mid-conversation.
|
*/

package prompts

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"

	"github.com/dimmihq/dimmi/app/enums"
	resprompts "github.com/dimmihq/dimmi/resources/prompts"
)

/*
DefaultVersion is the prompt version runs use unless configured or
pinned otherwise.
*/
const DefaultVersion = "v2"

/*
ErrInvalid marks a prompt tree that fails the boot check.
*/
var ErrInvalid = errors.New("prompts invalid")

/*
File is one resolved prompt file.
*/
type File struct {
	Name    string
	Path    string
	Content string
	Hash    string
}

/*
Fingerprint names everything that shaped a run's prompts: the version
and a hash per file used.
*/
type Fingerprint struct {
	Version string            `json:"version"`
	Hashes  map[string]string `json:"hashes"`
}

/*
Loader serves the checked prompt files of every compiled-in version.
It is immutable after NewLoader and safe for concurrent use.
*/
type Loader struct {
	version  string
	versions map[string]*set
}

/*
set is one version's files, keyed "<folder>/<name>", plus its parsed
default lines.
*/
type set struct {
	files    map[string]File
	defaults map[string]string
}

/*
Variables lists, per required file, the placeholders the pipeline
fills. A file naming any other placeholder fails the boot check.
*/
func Variables() map[string][]string {
	return map[string][]string{
		"answer.system.md": {
			"agent_name", "product_name", "tone", "language", "guidance", "channel", "handoff", "rules",
		},
		"tone_friendly.md":     {},
		"tone_professional.md": {},
		"tone_concise.md":      {},
		"defaults.md":          {},
	}
}

/*
DefaultKeys lists the lines defaults.md must define.
*/
func DefaultKeys() []string {
	return []string{"unknown", "handoff", "handoff_later", "handoff_unavailable", "max_replies", "error", "refusal"}
}

/*
NewLoader reads and checks every version in the compiled-in tree, with
an optional override tree that shadows it file by file. The configured
version must be among them; empty means DefaultVersion.
*/
func NewLoader(version string, override fs.FS) (*Loader, error) {
	if version == "" {
		version = DefaultVersion
	}

	trees := []fs.FS{resprompts.Files}
	if override != nil {
		trees = []fs.FS{override, resprompts.Files}
	}

	names, err := versionNames(resprompts.Files)
	if err != nil {
		return nil, err
	}

	l := &Loader{version: version, versions: map[string]*set{}}
	for _, name := range names {
		s, err := loadSet(trees, name)
		if err != nil {
			return nil, err
		}
		l.versions[name] = s
	}

	if _, ok := l.versions[version]; !ok {
		return nil, fmt.Errorf("%w: version %s is not compiled in", ErrInvalid, version)
	}
	return l, nil
}

/*
Version resolves the version a run uses: the requested one when it is
compiled in, the configured default otherwise. Runs record this value,
never the request.
*/
func (l *Loader) Version(requested string) string {
	if _, ok := l.versions[requested]; ok && requested != "" {
		return requested
	}
	return l.version
}

/*
Get resolves one file of the default version for the family.
*/
func (l *Loader) Get(family enums.AIModelFamily, name string) (*File, error) {
	return l.GetVersion(l.version, family, name)
}

/*
GetVersion resolves one file for the family: the family's folder
first, the common folder second.
*/
func (l *Loader) GetVersion(version string, family enums.AIModelFamily, name string) (*File, error) {
	s := l.versions[l.Version(version)]
	if f, ok := s.files[string(family)+"/"+name]; ok {
		return &f, nil
	}
	if f, ok := s.files["common/"+name]; ok {
		return &f, nil
	}
	return nil, fmt.Errorf("%w: %s not found", ErrInvalid, name)
}

/*
Defaults returns the version's default customer lines by key.
*/
func (l *Loader) Defaults(version string) map[string]string {
	return l.versions[l.Version(version)].defaults
}

/*
Fingerprint computes the hashes of the named files of a version.
*/
func (l *Loader) Fingerprint(version string, family enums.AIModelFamily, names []string) (*Fingerprint, error) {
	fp := &Fingerprint{Version: l.Version(version), Hashes: map[string]string{}}
	for _, name := range names {
		file, err := l.GetVersion(version, family, name)
		if err != nil {
			return nil, err
		}
		fp.Hashes[file.Path] = file.Hash
	}
	return fp, nil
}

/*
Versions lists the prompt versions compiled in.
*/
func Versions() ([]string, error) {
	return versionNames(resprompts.Files)
}

/*
versionNames lists the version folders of a tree.
*/
func versionNames(tree fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(tree, "agent")
	if err != nil {
		return nil, fmt.Errorf("read prompt versions: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

/*
loadSet reads one version: every required file of every folder, the
common folder holding them all, and the default lines.
*/
func loadSet(trees []fs.FS, version string) (*set, error) {
	folders, err := fs.ReadDir(trees[len(trees)-1], "agent/"+version)
	if err != nil {
		return nil, fmt.Errorf("read prompt version %s: %w", version, err)
	}

	s := &set{files: map[string]File{}}
	for _, folder := range folders {
		if !folder.IsDir() {
			continue
		}
		if err := s.loadFolder(trees, version, folder.Name()); err != nil {
			return nil, err
		}
	}

	for name := range Variables() {
		if _, ok := s.files["common/"+name]; !ok {
			return nil, fmt.Errorf("%w: %s/common/%s is missing", ErrInvalid, version, name)
		}
	}

	defaults, err := parseDefaults(s.files["common/defaults.md"].Content)
	if err != nil {
		return nil, fmt.Errorf("%w: %s defaults: %w", ErrInvalid, version, err)
	}
	s.defaults = defaults
	return s, nil
}

/*
loadFolder reads the required files present in one folder and checks
their placeholders.
*/
func (s *set) loadFolder(trees []fs.FS, version, folder string) error {
	for name, allowed := range Variables() {
		path := fmt.Sprintf("agent/%s/%s/%s", version, folder, name)
		content, ok := read(trees, path)
		if !ok {
			continue
		}
		if bad := unknownPlaceholders(content, allowed); len(bad) > 0 {
			return fmt.Errorf("%w: %s names unknown variables %s", ErrInvalid, path, strings.Join(bad, ", "))
		}
		s.files[folder+"/"+name] = File{Name: name, Path: path, Content: content, Hash: hash(content)}
	}
	return nil
}

/*
read loads a file from the first tree that has it.
*/
func read(trees []fs.FS, path string) (string, bool) {
	for _, tree := range trees {
		if data, err := fs.ReadFile(tree, path); err == nil {
			return string(data), true
		}
	}
	return "", false
}

/*
unknownPlaceholders lists the {{names}} in the text that are not
allowed, sorted.
*/
func unknownPlaceholders(text string, allowed []string) []string {
	var bad []string
	for _, m := range placeholderPattern().FindAllStringSubmatch(text, -1) {
		if !slices.Contains(allowed, m[1]) && !slices.Contains(bad, m[1]) {
			bad = append(bad, m[1])
		}
	}
	sort.Strings(bad)
	return bad
}

/*
placeholderPattern matches one {{name}}.
*/
func placeholderPattern() *regexp.Regexp {
	return regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)
}

/*
parseDefaults reads "key: text" lines; blank lines and lines starting
with # are skipped and a quoted text loses its quotes. Every key in
DefaultKeys must be present and non-empty.
*/
func parseDefaults(content string) (map[string]string, error) {
	out := map[string]string{}
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, text, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("line %q has no key", line)
		}
		text = strings.TrimSpace(text)
		if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
			text = text[1 : len(text)-1]
		}
		out[strings.TrimSpace(key)] = text
	}
	for _, key := range DefaultKeys() {
		if out[key] == "" {
			return nil, fmt.Errorf("key %s is missing", key)
		}
	}
	return out, nil
}

/*
hash is the short content fingerprint recorded on runs.
*/
func hash(content string) string {
	sum := hashing.SHA256([]byte(content))
	return hex.EncodeToString(sum[:8])
}
