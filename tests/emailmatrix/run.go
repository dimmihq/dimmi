//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| Running The Harness
|--------------------------------------------------------------------------
|
| One entry point takes loaded settings to a finished report on disk. Every
| run overwrites the latest report, and the one it replaces is what decides
| whether a host has filed mail as spam twice in a row.
|
*/

package emailmatrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
)

const (
	artifactJSON  = "tmp/emailmatrix/latest.json"
	artifactTable = "tmp/emailmatrix/latest.txt"
)

/*
execute runs what the flags ask for and writes the report. The returned error
says why the run does not count as passing.
*/
func execute(ctx context.Context, conf *settings, run flags, out io.Writer) error {
	clk := clock.System()

	disk, err := local.New(local.Options{Root: conf.Root})
	if err != nil {
		return fmt.Errorf("open root: %w", err)
	}

	api, err := newAPIProduct(conf.BaseURL, conf.APIToken)
	if err != nil {
		return err
	}

	boxes, err := openMailboxes(clk, conf.Mailboxes)
	if err != nil {
		return err
	}

	engine := &harness{
		Clock: clk, Product: api, Mailboxes: boxes, Forwarding: conf.Forwarding, AddressID: conf.AddressID,
		Wait: 5 * time.Minute, AnswerBar: time.Minute, Poll: 5 * time.Second,
	}

	finished := engine.runMatrixOrAdapter(ctx, conf, run, disk)

	return conclude(ctx, disk, finished, out)
}

func (h *harness) runMatrixOrAdapter(
	ctx context.Context, conf *settings, run flags, disk filesystem.Filesystem,
) report {
	if run.Adapter == "" {
		return h.runMatrix(ctx)
	}

	selected := adapterRun{
		Adapter:  adapterByName(run.Adapter),
		Target:   conf.Adapters[run.Adapter],
		From:     h.Mailboxes[0],
		Payloads: unavailablePayloads{},
	}

	if run.Record {
		selected.Disk = disk
	}

	return h.runAdapter(ctx, selected)
}

func conclude(ctx context.Context, disk filesystem.Filesystem, finished report, out io.Writer) error {
	previous, err := loadPrevious(ctx, disk)
	if err != nil {
		return err
	}

	var table strings.Builder
	if err := finished.writeTable(&table); err != nil {
		return err
	}

	if _, err := io.WriteString(out, table.String()); err != nil {
		return fmt.Errorf("print report: %w", err)
	}

	encoded, err := finished.json()
	if err != nil {
		return err
	}

	if err := disk.Put(ctx, artifactJSON, encoded, filesystem.PutOptions{}); err != nil {
		return fmt.Errorf("save report: %w", err)
	}

	if err := disk.Put(ctx, artifactTable, []byte(table.String()), filesystem.PutOptions{}); err != nil {
		return fmt.Errorf("save report: %w", err)
	}

	return verdict(finished, previous)
}

func loadPrevious(ctx context.Context, disk filesystem.Filesystem) (report, error) {
	contents, err := disk.Get(ctx, artifactJSON)
	if errors.Is(err, filesystem.ErrNotFound) {
		return report{}, nil
	}

	if err != nil {
		return report{}, fmt.Errorf("read previous report: %w", err)
	}

	var previous report
	if err := json.Unmarshal(contents, &previous); err != nil {
		return report{}, fmt.Errorf("decode previous report: %w", err)
	}

	return previous, nil
}

func verdict(finished, previous report) error {
	if twice := finished.spamTwice(previous); len(twice) > 0 {
		return fmt.Errorf("spam folder twice in a row on %s: release is held", strings.Join(twice, ", "))
	}

	if !finished.passed() {
		return errors.New("not every scenario passed")
	}

	return nil
}
