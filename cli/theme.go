/*
|--------------------------------------------------------------------------
| dimmi theme
|--------------------------------------------------------------------------
|
| Help center themes from the command line: start a theme from the
| default one, preview it with the help center's live content while
| writing it, and publish it. Every command works on a theme folder, the
| current one unless a folder is named.
|
*/

package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/cli/connect"
	"github.com/dimmihq/dimmi/cli/dimmi"
	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/themepreview"
	"github.com/dimmihq/dimmi/lang"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

const themeUsage = `dimmi theme builds your help center's own theme.

Usage:
  dimmi theme <command> [flags] [folder]

Commands:
  init         Start a theme in the folder from the default theme
  preview      Serve the help center locally in the theme, reloading on every save
  publish      Upload the theme, check it and make it live

Flags:
  --help-center ID   the help center (hc_...), the workspace's default one unless set
`

/*
theme finds the theme subcommand the arguments name and returns it
with the arguments left for its flags.
*/
func (a *app) theme(ctx context.Context, args []string) (command, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Print(themeUsage)
		return command{}, nil, nil
	}
	fs := newCommand("theme "+args[0], a)
	center := fs.String("help-center", "", "the help center (hc_...), the default one unless set")
	var run func(dir string) error
	switch args[0] {
	case "init":
		run = func(dir string) error { return a.themeInit(ctx, dir, *center) }
	case "preview":
		port := fs.Int("port", 4500, "the port to serve the preview on")
		noBrowser := fs.Bool("no-browser", false, "print the address instead of opening the browser")
		run = func(dir string) error {
			return a.themePreview(ctx, dir, previewOptions{center: *center, port: *port, noBrowser: *noBrowser})
		}
	case "publish":
		keep := fs.Bool("upload-only", false, "upload and check the theme without making it live")
		run = func(dir string) error {
			return a.themePublish(ctx, dir, publishOptions{center: *center, uploadOnly: *keep})
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command \"theme %s\"\n\n%s", args[0], themeUsage)
		return command{}, nil, errors.New("unknown command")
	}
	return command{fs, func() error { return run(folderArg(fs)) }}, args[1:], nil
}

/*
folderArg is the theme folder the command names, the current one when
it names none.
*/
func folderArg(fs *flag.FlagSet) string {
	if fs.NArg() > 0 {
		return fs.Arg(0)
	}
	return "."
}

/*
helpCenter finds the help center the flag names, or the workspace's
default one.
*/
func (a *app) helpCenter(ctx context.Context, client *dimmi.Client, id string) (dimmi.HelpCenter, error) {
	list, err := client.HelpCenters(ctx)
	if err != nil {
		return dimmi.HelpCenter{}, err
	}
	for _, h := range list {
		if h.ID == id || (id == "" && h.IsDefault) {
			return h, nil
		}
	}
	if id != "" {
		return dimmi.HelpCenter{}, fmt.Errorf("the workspace has no help center %s", id)
	}
	return dimmi.HelpCenter{}, errors.New("the workspace has no help center yet")
}

/*
themeInit writes the default theme into an empty or new folder, the
starting point of a theme of your own.
*/
func (a *app) themeInit(ctx context.Context, dir, center string) error {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty; start a theme in a new folder", dir)
	}
	client, err := a.destination()
	if err != nil {
		return err
	}
	h, err := a.helpCenter(ctx, client, center)
	if err != nil {
		return err
	}
	archive, err := client.ThemeArchive(ctx, h.ID, "default")
	if err != nil {
		return err
	}
	n, err := unzipInto(archive, dir)
	if err != nil {
		return err
	}
	a.report.Report(engine.Event{
		Event:   "theme_init",
		Message: fmt.Sprintf("Wrote the default theme's %d files to %s. Next: dimmi theme preview %s", n, dir, dir),
		Data:    map[string]any{"folder": dir, "files": n, "help_center": h.ID},
	})
	return nil
}

/*
unzipInto writes a theme's zip into the folder, keeping every path
inside it.
*/
func unzipInto(archive []byte, dir string) (int, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range r.File {
		target := filepath.Join(dir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) || f.FileInfo().IsDir() {
			continue
		}
		if err := writeZipped(f, target); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func writeZipped(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(io.LimitReader(rc, themes.MaxFileBytes+1))
	if err != nil {
		return err
	}
	return os.WriteFile(target, body, 0o600)
}

/*
themePreview serves the help center locally in the folder's theme,
with live content from dimmi, reloading pages on every save.
*/
/*
The preview's flags.
*/
type previewOptions struct {
	center    string
	port      int
	noBrowser bool
}

func (a *app) themePreview(ctx context.Context, dir string, opts previewOptions) error {
	center, port, noBrowser := opts.center, opts.port, opts.noBrowser
	client, err := a.destination()
	if err != nil {
		return err
	}
	h, err := a.helpCenter(ctx, client, center)
	if err != nil {
		return err
	}
	fallback, err := themes.LoadDefault()
	if err != nil {
		return err
	}
	tr, err := i18n.New(i18n.Config{Locale: i18n.Source}, lang.Files())
	if err != nil {
		return err
	}
	folder := &themepreview.Folder{Dir: dir, Fallback: fallback}
	if _, err := folder.Load(); err != nil {
		return err
	}
	srv := &themepreview.Server{Folder: folder, Pages: client, HelpCenter: h, DimmiURL: a.cfg.DimmiURL, Lang: tr}
	go folder.Watch(ctx, 300*time.Millisecond, func() {
		_, problems, _ := folder.State()
		a.reportProblems(problems)
		srv.Changed()
	})
	_, problems, _ := folder.State()
	a.reportProblems(problems)
	return srv.Listen(ctx, fmt.Sprintf("127.0.0.1:%d", port), func(addr string) {
		a.report.Report(engine.Event{
			Event: "theme_preview", Message: "Previewing " + h.Name + " at " + addr + " (ctrl-c to stop)",
			Data: map[string]any{"url": addr, "help_center": h.ID},
		})
		if !noBrowser {
			_ = connect.OpenBrowser(ctx, addr)
		}
	})
}

/*
reportProblems prints what is wrong with the theme, or that it loads.
*/
func (a *app) reportProblems(problems []themes.Problem) {
	if len(problems) == 0 {
		a.say("The theme loads with no problems.")
		return
	}
	for _, p := range problems {
		a.report.Report(engine.Event{Event: "theme_problem", Message: p.String(), Data: p})
	}
}

/*
themePublish uploads the folder as the help center's next theme version
and makes it live, unless asked only to upload it.
*/
/*
The publish command's flags.
*/
type publishOptions struct {
	center     string
	uploadOnly bool
}

func (a *app) themePublish(ctx context.Context, dir string, opts publishOptions) error {
	center, uploadOnly := opts.center, opts.uploadOnly
	files, err := themepreview.ReadFolder(dir)
	if err != nil {
		return err
	}
	archive, err := themepreview.Zip(files)
	if err != nil {
		return err
	}
	client, err := a.destination()
	if err != nil {
		return err
	}
	h, err := a.helpCenter(ctx, client, center)
	if err != nil {
		return err
	}
	theme, err := client.UploadTheme(ctx, h.ID, archive)
	var refused *dimmi.Error
	if errors.As(err, &refused) && len(refused.Problems) > 0 {
		for _, p := range refused.Problems {
			problem := themes.Problem{File: p.File, Line: p.Line, Message: p.Message}
			a.report.Report(engine.Event{Event: "theme_problem", Message: problem.String(), Data: problem})
		}
		return errors.New("dimmi refused the theme; nothing was uploaded")
	}
	if err != nil {
		return err
	}
	if !uploadOnly {
		if err := client.PublishTheme(ctx, h.ID, theme.ID); err != nil {
			return err
		}
	}
	verb := "Published"
	if uploadOnly {
		verb = "Uploaded"
	}
	a.report.Report(engine.Event{
		Event: "theme_published",
		Message: fmt.Sprintf("%s %s as version %d of %s's theme (%s).",
			verb, theme.Name, theme.Version, h.Name, theme.ID),
		Data: map[string]any{"theme": theme.ID, "version": theme.Version, "help_center": h.ID, "live": !uploadOnly},
	})
	return nil
}
