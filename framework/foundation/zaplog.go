/*
|--------------------------------------------------------------------------
| Library Log Bridge
|--------------------------------------------------------------------------
|
| Some libraries log through zap instead of the standard structured
| logger. Their lines are handed to the application's logger, so they
| land in the same stream, with the same format, level and redaction as
| every other line.
|
*/

package foundation

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dimmihq/dimmi/framework/log"
)

/*
zapLogger returns a zap logger whose lines go to the logger.
*/
func zapLogger(logger log.Logger) *zap.Logger {
	return zap.New(slogCore{logger: logger})
}

/*
slogCore is a zap core that writes each entry to a structured logger,
with the fields gathered so far.
*/
type slogCore struct {
	logger log.Logger
	attrs  []log.Attr
}

/*
Enabled reports whether the logger writes lines at the level.
*/
func (c slogCore) Enabled(level zapcore.Level) bool {
	return c.logger.Enabled(context.Background(), slogLevel(level))
}

/*
With returns a core that adds the fields to every line.
*/
func (c slogCore) With(fields []zapcore.Field) zapcore.Core {
	return slogCore{logger: c.logger, attrs: append(append([]log.Attr{}, c.attrs...), toAttrs(fields)...)}
}

/*
Check adds this core to the entry when its level is written.
*/
func (c slogCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(entry.Level) {
		return checked
	}

	return checked.AddCore(entry, c)
}

/*
Write hands the entry, with its own time and fields, to the logger's
handler, which formats and redacts it like every other line.
*/
func (c slogCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	record := slog.NewRecord(entry.Time, slogLevel(entry.Level), entry.Message, 0)
	record.AddAttrs(c.attrs...)
	record.AddAttrs(toAttrs(fields)...)

	if entry.LoggerName != "" {
		record.AddAttrs(log.String("logger", entry.LoggerName))
	}

	if err := c.logger.Handler().Handle(context.Background(), record); err != nil {
		return fmt.Errorf("write library log line: %w", err)
	}

	return nil
}

/*
Sync does nothing: the logger writes each line as it comes.
*/
func (slogCore) Sync() error {
	return nil
}

/*
toAttrs turns zap fields into attributes, letting zap encode each value.
*/
func toAttrs(fields []zapcore.Field) []log.Attr {
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(encoder)
	}

	attrs := make([]log.Attr, 0, len(encoder.Fields))
	for key, value := range encoder.Fields {
		attrs = append(attrs, log.Any(key, value))
	}

	return attrs
}

/*
slogLevel maps a zap level to the nearest structured log level.
*/
func slogLevel(level zapcore.Level) slog.Level {
	switch {
	case level <= zapcore.DebugLevel:
		return slog.LevelDebug
	case level == zapcore.InfoLevel:
		return slog.LevelInfo
	case level == zapcore.WarnLevel:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}
