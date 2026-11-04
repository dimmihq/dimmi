/*
|--------------------------------------------------------------------------
| Health Command
|--------------------------------------------------------------------------
|
| A container image without a shell or curl still needs a health check
| its orchestrator can run. The binary asks its own admin listener for
| readiness and exits 0 when ready, 1 otherwise, for any role: web and
| worker processes both serve the admin listener.
|
*/

package foundation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
ErrNotReady is the health command's failure: the process answered but
is not ready, or did not answer.
*/
var ErrNotReady = errors.New("foundation: not ready")

/*
healthTimeout bounds the health command, well inside any orchestrator's
own check timeout.
*/
const healthTimeout = 4 * time.Second

func (k *kernel) healthCommand() console.Command {
	return command{
		name:        "health",
		description: "Check the running process through its admin listener",
		handle: func(ctx context.Context, _ *console.Input, out *console.Output) error {
			cfg, err := k.config(ctx)
			if err != nil {
				return err
			}

			if err := checkReady(ctx, cfg.HTTP.AdminAddr); err != nil {
				return err
			}

			out.Info("ready")

			return nil
		},
	}
}

/*
checkReady asks the admin listener at the address for readiness. A
listener bound to every interface is reached on loopback.
*/
func checkReady(ctx context.Context, addr string) error {
	if addr == "" {
		return fmt.Errorf("%w: http.admin_addr is empty, so there is nothing to ask", ErrNotReady)
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%w: http.admin_addr %q: %w", ErrNotReady, addr, err)
	}

	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true, Timeout: healthTimeout})
	if err != nil {
		return fmt.Errorf("health client: %w", err)
	}

	response, err := client.New().Get(ctx, "http://"+net.JoinHostPort(host, port)+PathReady)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}

	if response.Status != http.StatusOK {
		return fmt.Errorf("%w: %s answered %d", ErrNotReady, PathReady, response.Status)
	}

	return nil
}
