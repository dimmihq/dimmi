package console_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/console"
)

/*
handler is a command running the function, to test output helpers.
*/
type handler func(ctx context.Context, in *console.Input, out *console.Output) error

func (handler) Name() string {
	return "try"
}

func (handler) Description() string {
	return "Try an output helper"
}

func (handler) Flags(*console.Flags) {}

func (h handler) Handle(ctx context.Context, in *console.Input, out *console.Output) error {
	return h(ctx, in, out)
}

/*
confirm returns a command that asks the question and prints the answer.
*/
func confirm(fallback bool) handler {
	return func(ctx context.Context, _ *console.Input, out *console.Output) error {
		answer, err := out.Confirm(ctx, "Overwrite the file?", fallback)
		if err != nil {
			return err
		}

		out.Line("answer=%t", answer)

		return nil
	}
}

func TestOutput(t *testing.T) {
	t.Parallel()

	t.Run("writes a table with headers", func(t *testing.T) {
		t.Parallel()
		table := handler(func(_ context.Context, _ *console.Input, out *console.Output) error {
			out.Table([]string{"Check", "Status"}, [][]string{{"database", "pass"}, {"valkey", "fail"}})

			return nil
		})

		_, stdout, _ := run(t, console.IO{}, []string{"try"}, table)
		_, quiet, _ := run(t, console.IO{}, []string{"try", "-q"}, table)

		assert.Contains(t, stdout, "│ Check    │ Status │")
		assert.Contains(t, stdout, "│ valkey   │ fail   │")
		assert.Empty(t, quiet)
	})

	t.Run("writes only the final count of a progress bar outside a terminal", func(t *testing.T) {
		t.Parallel()
		progress := handler(func(_ context.Context, _ *console.Input, out *console.Output) error {
			bar := out.Progress(4)
			bar.Advance(1)
			bar.Advance(5)
			bar.Finish()

			return nil
		})

		_, stdout, _ := run(t, console.IO{}, []string{"try"}, progress)
		_, quiet, _ := run(t, console.IO{}, []string{"try", "-q"}, progress)

		assert.Equal(t, "["+strings.Repeat("=", 30)+"] 4/4\n", stdout)
		assert.Empty(t, quiet)
	})

	t.Run("takes the default answer without someone to ask", func(t *testing.T) {
		t.Parallel()

		_, yes, _ := run(t, console.IO{}, []string{"try"}, confirm(true))
		_, no, _ := run(t, console.IO{Interactive: true}, []string{"try", "-n"}, confirm(false))

		assert.Equal(t, "answer=true\n", yes)
		assert.Equal(t, "answer=false\n", no)
	})

	t.Run("asks the question when someone can answer", func(t *testing.T) {
		t.Parallel()

		answering := func(answer string) console.IO {
			return console.IO{Interactive: true, In: strings.NewReader(answer + "\n")}
		}

		_, yes, _ := run(t, answering("y"), []string{"try"}, confirm(false))
		_, no, _ := run(t, answering("n"), []string{"try"}, confirm(true))

		assert.Contains(t, yes, "Overwrite the file?")
		assert.Contains(t, yes, "answer=true")
		assert.Contains(t, no, "answer=false")
	})
}
