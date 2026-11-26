package connect

import (
	"context"
	"os/exec"
	"runtime"
)

/*
OpenBrowser opens the address in the default browser. It fails quietly
on a machine without one; the address is printed either way. The opener
keeps running when ctx ends.
*/
func OpenBrowser(ctx context.Context, u string) error {
	if !Valid(u) {
		return nil
	}

	name, args := "xdg-open", []string{u}

	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", u}
	}

	// The opener runs on after the CLI moves on, so it has no deadline.
	opener := context.WithoutCancel(ctx)
	cmd := exec.CommandContext(opener, name, args...) //nolint:gosec // fixed opener, u is a checked URL

	return cmd.Start()
}
