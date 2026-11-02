/*
|--------------------------------------------------------------------------
| Logger
|--------------------------------------------------------------------------
|
| Every line is structured: a short static message plus typed attributes,
| so lines can be searched and counted instead of parsed. Servers write
| JSON to stdout for the platform to collect; local development writes
| plain text a person can read.
|
| Lines below the configured level cost almost nothing, and no secret
| reaches the output: every attribute passes through redaction first.
|
*/

package log

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
)

/*
Logger writes log lines. The application holds it without importing
log/slog and always logs with a context, such as InfoContext, so fields
carried by the context land on the line.
*/
type Logger = *slog.Logger

/*
Errors from building a logger: a driver other than json or text, or a
level that is not debug, info, warn or error.
*/
var (
	ErrUnknownDriver = errors.New("log: unknown driver")
	ErrInvalidLevel  = errors.New("log: invalid level")
)

/*
Options choose how and where lines are written.

Driver is json for servers or text for local development. Level is the
lowest level written. Redact names extra attribute keys and struct
fields whose values are hidden, on top of the built-in secret names.
Writer receives the lines and is stdout when left nil; it is set in
code, never from configuration.
*/
type Options struct {
	Driver string    `default:"json" koanf:"driver"`
	Level  string    `default:"info" koanf:"level"`
	Redact []string  `koanf:"redact"`
	Writer io.Writer `koanf:"-"`
}

/*
New returns a logger that writes lines as the options say.
*/
func New(opts Options) (Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(opts.Level)); err != nil {
		return nil, fmt.Errorf("level %q: %w", opts.Level, ErrInvalidLevel)
	}

	w := opts.Writer
	if w == nil {
		w = os.Stdout
	}

	handlerOpts := &slog.HandlerOptions{Level: level, ReplaceAttr: redactor(opts.Redact)}

	switch opts.Driver {
	case "json":
		return slog.New(contextHandler{Handler: slog.NewJSONHandler(w, handlerOpts)}), nil
	case "text":
		return slog.New(contextHandler{Handler: slog.NewTextHandler(w, handlerOpts)}), nil
	}

	return nil, fmt.Errorf("driver %q: %w", opts.Driver, ErrUnknownDriver)
}
