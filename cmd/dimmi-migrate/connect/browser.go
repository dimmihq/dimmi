package connect

import (
	"os/exec"
	"runtime"
)

/*
OpenBrowser opens the address in the default browser. It fails quietly
on a machine without one; the address is printed either way.
*/
func OpenBrowser(u string) error {
	if !Valid(u) {
		return nil
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}

	return cmd.Start()
}
