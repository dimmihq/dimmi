/*
|--------------------------------------------------------------------------
| Stdio Transport
|--------------------------------------------------------------------------
|
| Local agents start the command line program and talk to it over its
| standard input and output, one JSON message per line. The process is
| one caller: the context the server runs in carries that caller and the
| toolsets they enabled, decided once when the program starts. Nothing
| but protocol messages may reach standard output.
|
*/

package mcp

import (
	"context"
	"fmt"
	"io"
	"os"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

/*
ServeStdio serves the server over the process's standard input and
output until the client closes input or the context ends. The context
carries the caller, as auth stores it, and the toolsets from
WithToolsets.
*/
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.Serve(ctx, os.Stdin, os.Stdout)
}

/*
Serve serves the server over a pair of streams carrying one JSON message
per line, as ServeStdio does over the process's own.
*/
func (s *Server) Serve(ctx context.Context, in io.ReadCloser, out io.WriteCloser) error {
	if err := s.sdk.Run(ctx, &sdk.IOTransport{Reader: in, Writer: out}); err != nil {
		return fmt.Errorf("serve mcp: %w", err)
	}

	return nil
}
