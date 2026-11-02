/*
|--------------------------------------------------------------------------
| Make Commands
|--------------------------------------------------------------------------
|
| Every kind of file gets its own command, make:<kind>, taking the name
| and --force to overwrite a file that exists. The first file of a new
| package also gets the package's doc.go.
|
*/

package make

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
)

/*
command is the make command of one kind.
*/
type command struct {
	generator Generator
	kind      kind
}

/*
Name returns make: followed by the kind.
*/
func (c *command) Name() string {
	return "make:" + c.kind.name
}

/*
Description returns the kind's one-line description.
*/
func (c *command) Description() string {
	return c.kind.description
}

/*
Flags declares the name, --force and the kind's own flags.
*/
func (c *command) Flags(flags *console.Flags) {
	flags.Argument("name", c.kind.argument)
	flags.Bool("force", "overwrite the file if it exists")

	if c.kind.flags != nil {
		c.kind.flags(flags)
	}
}

/*
Handle plans the file, writes it and prints its path.
*/
func (c *command) Handle(_ context.Context, in *console.Input, out *console.Output) error {
	n, err := parseName(in.Argument("name"))
	if err != nil {
		return err
	}

	f, err := c.kind.plan(n, in, c.generator)
	if err != nil {
		return err
	}

	if c.generator.exists(f) && !in.Bool("force") {
		return fmt.Errorf("%w: %s; pass --force to replace it", ErrExists, f.path)
	}

	written, err := c.generator.write(f)
	if err != nil {
		return err
	}

	out.Info("Created %s", written)

	return c.writePackageDoc(f, out)
}

/*
writePackageDoc adds the package's doc.go when the file starts a new Go
package.
*/
func (c *command) writePackageDoc(f file, out *console.Output) error {
	if c.kind.holds == "" {
		return nil
	}

	doc := packageDoc(f, c.kind.holds)
	if c.generator.exists(doc) {
		return nil
	}

	written, err := c.generator.write(doc)
	if err != nil {
		return err
	}

	out.Info("Created %s", written)

	return nil
}
