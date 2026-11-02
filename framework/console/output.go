/*
|--------------------------------------------------------------------------
| Output
|--------------------------------------------------------------------------
|
| Commands write through Output so that every command looks the same and
| behaves well wherever it runs. Colors appear only on a terminal that
| supports them, never in a pipe or a log file. Quiet runs print nothing
| but errors, and errors go to the error stream so scripts can tell them
| apart from results.
|
| Questions are asked only when someone is there to answer. Otherwise,
| as in scripts, CI or with --no-interaction, every question takes its
| default answer without waiting.
|
*/

package console

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

/*
Output writes a command's messages and asks its questions.
*/
type Output struct {
	in          io.Reader
	out         io.Writer
	err         io.Writer
	quiet       bool
	interactive bool
	terminal    bool

	info    lipgloss.Style
	warn    lipgloss.Style
	failure lipgloss.Style
	header  lipgloss.Style
}

/*
newOutput returns output on the streams, styled for what each stream
supports.
*/
func newOutput(streams IO, quiet, interactive bool) *Output {
	out := lipgloss.NewRenderer(streams.Out)
	errs := lipgloss.NewRenderer(streams.Err)

	return &Output{
		in:          streams.In,
		out:         streams.Out,
		err:         streams.Err,
		quiet:       quiet,
		interactive: interactive,
		terminal:    isTerminal(streams.Out),
		info:        out.NewStyle().Foreground(lipgloss.Color("2")),
		warn:        out.NewStyle().Foreground(lipgloss.Color("3")),
		failure:     errs.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		header:      out.NewStyle().Bold(true),
	}
}

/*
Line writes a plain line.
*/
func (o *Output) Line(format string, args ...any) {
	if o.quiet {
		return
	}

	_, _ = fmt.Fprintf(o.out, format+"\n", args...)
}

/*
Info writes a line saying something went as planned.
*/
func (o *Output) Info(format string, args ...any) {
	o.styled(o.info, format, args...)
}

/*
Warn writes a line about something worth a look that did not stop the
command.
*/
func (o *Output) Warn(format string, args ...any) {
	o.styled(o.warn, format, args...)
}

/*
Error writes a line to the error stream, even in a quiet run.
*/
func (o *Output) Error(format string, args ...any) {
	_, _ = fmt.Fprintln(o.err, o.failure.Render(fmt.Sprintf(format, args...)))
}

/*
Table writes rows under bold headers, in aligned columns.
*/
func (o *Output) Table(headers []string, rows [][]string) {
	if o.quiet {
		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return o.header.Padding(0, 1)
			}

			return lipgloss.NewStyle().Padding(0, 1)
		})

	_, _ = fmt.Fprintln(o.out, t.Render())
}

/*
Confirm asks a yes or no question and returns the answer. Without
someone to answer it returns the default at once.
*/
func (o *Output) Confirm(ctx context.Context, question string, fallback bool) (bool, error) {
	if !o.interactive {
		return fallback, nil
	}

	answer := fallback
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(question).Value(&answer))).
		WithInput(o.in).
		WithOutput(o.out).
		WithShowHelp(false).
		WithAccessible(!isTerminal(o.in))

	if err := form.RunWithContext(ctx); err != nil {
		return fallback, fmt.Errorf("ask %q: %w", question, err)
	}

	return answer, nil
}

func (o *Output) styled(style lipgloss.Style, format string, args ...any) {
	if o.quiet {
		return
	}

	_, _ = fmt.Fprintln(o.out, style.Render(fmt.Sprintf(format, args...)))
}

/*
isTerminal reports whether the stream is a character device, such as a
terminal, rather than a pipe, a file or a buffer.
*/
func isTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()

	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

/*
Progress shows how far a long task has come. On a terminal it redraws
one bar in place; elsewhere it writes only the final count.
*/
type Progress struct {
	output *Output
	total  int
	done   int
}

/*
progressWidth is how many cells the bar takes.
*/
const progressWidth = 30

/*
Progress starts a bar for a task of total steps.
*/
func (o *Output) Progress(total int) *Progress {
	return &Progress{output: o, total: max(total, 1)}
}

/*
Advance records steps done and redraws the bar.
*/
func (p *Progress) Advance(steps int) {
	p.done = min(p.done+steps, p.total)

	if p.output.quiet || !p.output.terminal {
		return
	}

	_, _ = fmt.Fprint(p.output.out, "\r"+p.bar())
}

/*
Finish completes the bar and ends its line.
*/
func (p *Progress) Finish() {
	p.done = p.total

	if p.output.quiet {
		return
	}

	if p.output.terminal {
		_, _ = fmt.Fprint(p.output.out, "\r")
	}

	_, _ = fmt.Fprintln(p.output.out, p.bar())
}

func (p *Progress) bar() string {
	filled := progressWidth * p.done / p.total

	return fmt.Sprintf("[%s%s] %d/%d",
		strings.Repeat("=", filled), strings.Repeat(" ", progressWidth-filled), p.done, p.total)
}
