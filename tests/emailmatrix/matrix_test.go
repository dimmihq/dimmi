//go:build emailmatrix

package emailmatrix

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/process"
)

/*
TestEmailMatrix is the real run, started by make email-matrix. It sends real
mail, so it refuses before doing anything when any credential is absent.
*/
func TestEmailMatrix(t *testing.T) {
	conf, run, err := loadSettings(t.Context(), nil, awsCLISecrets{runner: process.New()})
	require.NoError(t, err)

	require.NoError(t, execute(t.Context(), conf, run, t.Output()))
}
