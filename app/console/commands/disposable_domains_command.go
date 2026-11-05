//go:build dev

/*
|--------------------------------------------------------------------------
| Refresh Disposable Domains Command
|--------------------------------------------------------------------------
|
| The list of throwaway mailbox domains is embedded in the binary, so a
| running server never fetches it. A developer refreshes it from its
| public, CC0 dedicated source with this command and commits the new
| file; the next build carries it.
|
*/

package commands

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
)

/*
The source of the list and the file it is stored in.
*/
const (
	disposableDomainsSource = "https://raw.githubusercontent.com/disposable-email-domains/" +
		"disposable-email-domains/main/disposable_email_blocklist.conf"
	disposableDomainsDir  = "resources/data"
	disposableDomainsFile = "disposable_email_domains.txt.gz"
)

/*
DisposableDomainsCommand refreshes the embedded list of throwaway
mailbox domains.
*/
type DisposableDomainsCommand struct {
	App *app.App
}

/*
Name returns data:disposable-domains.
*/
func (DisposableDomainsCommand) Name() string {
	return "data:disposable-domains"
}

/*
Description says what the command does.
*/
func (DisposableDomainsCommand) Description() string {
	return "Refresh the embedded list of throwaway mailbox domains"
}

/*
Flags declares nothing.
*/
func (DisposableDomainsCommand) Flags(*console.Flags) {}

/*
Handle downloads the list, keeps one lower-case domain per line, and
stores it compressed.
*/
func (c DisposableDomainsCommand) Handle(
	ctx context.Context, _ *foundation.Application, _ *console.Input, out *console.Output,
) error {
	resp, err := c.App.HTTP.New().MaxBody(16<<20).Get(ctx, disposableDomainsSource)
	if err != nil {
		return fmt.Errorf("download disposable domains: %w", err)
	}

	var list strings.Builder

	count := 0

	for line := range strings.Lines(string(resp.Body)) {
		if domain := strings.ToLower(strings.TrimSpace(line)); domain != "" && !strings.HasPrefix(domain, "#") {
			list.WriteString(domain + "\n")
			count++
		}
	}

	var packed bytes.Buffer

	zw, err := gzip.NewWriterLevel(&packed, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("compress disposable domains: %w", err)
	}

	if _, err := zw.Write([]byte(list.String())); err != nil {
		return fmt.Errorf("compress disposable domains: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("compress disposable domains: %w", err)
	}

	disk, err := local.New(local.Options{Root: disposableDomainsDir})
	if err != nil {
		return fmt.Errorf("open %s: %w", disposableDomainsDir, err)
	}

	if err := disk.Put(ctx, disposableDomainsFile, packed.Bytes(), filesystem.PutOptions{}); err != nil {
		return fmt.Errorf("write disposable domains: %w", err)
	}

	out.Info("Stored %d domains in %s/%s.", count, disposableDomainsDir, disposableDomainsFile)

	return nil
}
