package csvsafe_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/csvsafe"
)

/*
brokenPipe is a destination that refuses every write.
*/
type brokenPipe struct{}

func (brokenPipe) Write([]byte) (int, error) {
	return 0, errors.New("broken pipe")
}

func TestCell(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in, want string
	}{
		{"guards an equals sign", "=SUM(A1:A2)", "'=SUM(A1:A2)"},
		{"guards a plus sign", "+1", "'+1"},
		{"guards a minus sign", "-2+3", "'-2+3"},
		{"guards an at sign", "@cmd", "'@cmd"},
		{"guards a leading tab", "\t=1", "'\t=1"},
		{"guards a leading carriage return", "\r=1", "'\r=1"},
		{"leaves plain text alone", "Ada Lovelace", "Ada Lovelace"},
		{"leaves a sign inside the text alone", "a=b", "a=b"},
		{"leaves an empty cell empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, csvsafe.Cell(tc.in))
		})
	}
}

func TestWriter(t *testing.T) {
	t.Parallel()

	t.Run("writes quoted records with every cell guarded", func(t *testing.T) {
		t.Parallel()
		var out strings.Builder
		w := csvsafe.NewWriter(&out)

		require.NoError(t, w.Write([]string{"name", "note"}))
		require.NoError(t, w.Write([]string{"=HYPERLINK(\"x\")", "a, b"}))
		require.NoError(t, w.Flush())

		assert.Equal(t, "name,note\n\"'=HYPERLINK(\"\"x\"\")\",\"a, b\"\n", out.String())
	})

	t.Run("reports a destination that refuses the data", func(t *testing.T) {
		t.Parallel()
		w := csvsafe.NewWriter(brokenPipe{})

		require.NoError(t, w.Write([]string{"a"}))
		require.ErrorContains(t, w.Flush(), "broken pipe")
	})

	t.Run("reports a record it cannot write", func(t *testing.T) {
		t.Parallel()
		w := csvsafe.NewWriter(brokenPipe{})

		_ = w.Write([]string{strings.Repeat("x", 5000)})

		require.Error(t, w.Write([]string{"y"}))
	})
}
