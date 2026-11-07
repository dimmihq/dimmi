/*
|--------------------------------------------------------------------------
| Lovelace Actions Demo
|--------------------------------------------------------------------------
|
| One process plays the customer-side systems of Lovelace, a fictional
| SaaS, so an agent's actions can be tried end to end without a real
| billing system: a REST API on one port and an MCP server on another,
| both over the same in-memory records. Nothing here is part of any
| product binary; it is a stand-in for the systems a business already
| has.
|
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

/*
config is where the two servers listen and the bearer token each
requires.
*/
type config struct {
	apiAddr  string
	mcpAddr  string
	token    string
	mcpToken string
}

func main() {
	cfg := parseFlags(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, cfg, os.Stdout)
	stop()

	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "actions-demo:", err)
		os.Exit(1)
	}
}

/*
parseFlags reads the command line. Tokens default to the environment,
then to the demo values the README uses.
*/
func parseFlags(args []string) config {
	var cfg config

	flags := flag.NewFlagSet("actions-demo", flag.ExitOnError)
	flags.StringVar(&cfg.apiAddr, "addr", ":4100", "address of the REST API")
	flags.StringVar(&cfg.mcpAddr, "mcp-addr", ":4200", "address of the MCP server")
	flags.StringVar(&cfg.token, "token", envOr("LOVELACE_API_TOKEN", "demo-token"), "bearer token of the REST API")
	flags.StringVar(&cfg.mcpToken, "mcp-token", envOr("LOVELACE_MCP_TOKEN", "demo-mcp-token"),
		"bearer token of the MCP server")
	_ = flags.Parse(args)

	return cfg
}

func envOr(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}

	return fallback
}

/*
run serves the REST API and the MCP server until the context ends or
either server fails, then stops both.
*/
func run(ctx context.Context, cfg config, out io.Writer) error {
	st := newStore(time.Now)
	j := newJournal(out, time.Now)

	mcpHandler, err := newMCPHandler(st, j, cfg.mcpToken)
	if err != nil {
		return fmt.Errorf("build mcp server: %w", err)
	}

	servers := []*http.Server{
		{
			Addr: cfg.apiAddr, Handler: (&restAPI{store: st, journal: j, token: cfg.token}).routes(),
			ReadHeaderTimeout: 10 * time.Second,
		},
		{Addr: cfg.mcpAddr, Handler: mcpHandler, ReadHeaderTimeout: 10 * time.Second},
	}

	errs := make(chan error, len(servers))
	for _, srv := range servers {
		go func() { errs <- srv.ListenAndServe() }()
	}

	announce(j, cfg)

	select {
	case <-ctx.Done():
		err = nil
	case err = <-errs:
	}

	shutdown(ctx, servers)

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

/*
shutdown gives in-flight requests a few seconds to finish.
*/
func shutdown(ctx context.Context, servers []*http.Server) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	for _, srv := range servers {
		_ = srv.Shutdown(ctx)
	}
}

/*
announce prints where everything is, so the demo can be driven straight
from the startup output.
*/
func announce(j *journal, cfg config) {
	j.line("Lovelace actions demo")
	j.line("  REST API    %s/api   Authorization: Bearer %s", baseURL(cfg.apiAddr), cfg.token)
	j.line("  MCP server  %s/mcp   Authorization: Bearer %s", baseURL(cfg.mcpAddr), cfg.mcpToken)
	j.line("  Customers   user_42 Ada (Pro annual, refundable), user_7 Grace (Pro monthly), " +
		"user_99 Katherine (Business annual, past 30 days)")
	j.line("")
}

/*
baseURL turns a listen address into the URL a local client would use,
reading an address without a host as localhost.
*/
func baseURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}

	return "http://" + addr
}
