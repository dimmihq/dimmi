/*
|--------------------------------------------------------------------------
| File Generator
|--------------------------------------------------------------------------
|
| New files of a known kind always start the same way, so they are
| generated rather than typed: the right directory, file name, package,
| type names and docblocks, rendered from a stub that already passes the
| linter. The application may override any stub with its own file of the
| same name.
|
| A file that exists is never overwritten unless the caller forces it,
| so running a generator twice cannot lose work. Stubs are written in
| gofmt layout, since the output is not reformatted.
|
*/

package make

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/console"
)

/*
ErrExists is returned for a file that already exists when the caller
did not force overwriting it.
*/
var ErrExists = errors.New("make: file already exists")

/*
Generator renders stubs into the project.

Stubs holds the default stubs and Overrides the application's own,
either may be nil. Module is the application's module path and Framework
the import path of the framework, Module + "/framework" when empty. Root
is the project directory files are written under. Clock stamps migration
versions.
*/
type Generator struct {
	Stubs     fs.FS
	Overrides fs.FS
	Module    string
	Framework string
	Root      string
	Clock     clock.Clock
}

/*
Data is what a stub is rendered with.

Package is the Go package of the new file and Holds what that package
holds, for its doc comment. Name is the type name, Snake the same in
snake case and Title in words. Event names the event type a listener
handles, Signature what a command is called with, and Table the table a
migration creates. Authorize and Validate ask a request for its hooks.
Resource is the type a policy decides about and Model its type in
app/models, when it names one; Permission prefixes the policy's
permissions. Each is empty where it does not apply.
*/
type Data struct {
	Module     string
	Framework  string
	Package    string
	Holds      string
	Name       string
	Snake      string
	Title      string
	Event      string
	Signature  string
	Table      string
	Authorize  bool
	Validate   bool
	Model      string
	Resource   string
	Permission string
}

/*
file is one file to generate: where it goes, from which stub and with
what data.
*/
type file struct {
	path string
	stub string
	data Data
}

/*
Commands returns the make commands, one per kind of file.
*/
func (g Generator) Commands() []console.Command {
	commands := make([]console.Command, 0, len(kinds()))
	for _, k := range kinds() {
		commands = append(commands, &command{generator: g, kind: k})
	}

	return commands
}

/*
exists reports whether the file is already in the project.
*/
func (g Generator) exists(f file) bool {
	_, err := os.Stat(g.target(f))

	return err == nil
}

/*
write renders the file and writes it under Root, replacing any file
already there, and returns its path relative to Root.
*/
func (g Generator) write(f file) (string, error) {
	f.data.Module = g.Module
	f.data.Framework = cmp.Or(g.Framework, g.Module+"/framework")

	source, err := g.render(f)
	if err != nil {
		return "", err
	}

	target := g.target(f)
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return "", fmt.Errorf("create directory for %s: %w", f.path, err)
	}

	if err := os.WriteFile(target, source, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", f.path, err)
	}

	return f.path, nil
}

/*
packageDoc returns the doc.go of the file's package, which every new
package needs before it passes the linter.
*/
func packageDoc(f file, holds string) file {
	return file{
		path: path.Join(path.Dir(f.path), "doc.go"),
		stub: "doc.go.tmpl",
		data: Data{Package: f.data.Package, Holds: holds},
	}
}

func (g Generator) target(f file) string {
	return filepath.Join(g.Root, filepath.FromSlash(f.path))
}

/*
render fills the stub with the data.
*/
func (g Generator) render(f file) ([]byte, error) {
	text, err := g.stub(f.stub)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(f.stub).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse stub %s: %w", f.stub, err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, f.data); err != nil {
		return nil, fmt.Errorf("render stub %s: %w", f.stub, err)
	}

	return out.Bytes(), nil
}

/*
stub returns the application's override of the named stub, or the
default when there is none.
*/
func (g Generator) stub(name string) (string, error) {
	for _, stubs := range []fs.FS{g.Overrides, g.Stubs} {
		if stubs == nil {
			continue
		}

		text, err := fs.ReadFile(stubs, name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return "", fmt.Errorf("read stub %s: %w", name, err)
		}

		return string(text), nil
	}

	return "", fmt.Errorf("read stub %s: %w", name, fs.ErrNotExist)
}
