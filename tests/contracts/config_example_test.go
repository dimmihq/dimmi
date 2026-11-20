/*
|--------------------------------------------------------------------------
| The Example Configuration
|--------------------------------------------------------------------------
|
| dimmi.example.yaml is what a newcomer copies to dimmi.yaml, so it lists
| every setting the application reads, each at its default beside the
| DIMMI_ variable that sets the same value, under what its struct's
| comment says. It is written from the configuration structs; the values
| it sets for local development are kept. A setting added to a struct
| fails this test until `make config-example` writes the file again, and
| the file must load as a configuration, so it stays one a newcomer can
| run.
|
*/

package contracts_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	fconfig "github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/config"
)

const (
	repositoryRoot = "../.."
	examplePath    = repositoryRoot + "/dimmi.example.yaml"
	exampleHeader  = `Every setting dimmi reads, each at its default, with the environment
variable that sets the same value beside it. ` + "`make dev`" + ` copies this file
to dimmi.yaml, which stays out of git: edit that copy. The values set
here for local development point at the services compose.yaml starts
(` + "`make services`" + `).

A variable wins over this file. A secret can also come from a file:
name it in the variable with _FILE added.

This file is written from the configuration structs. After changing a
value here or adding a setting to a struct, run ` + "`make config-example`."
)

func TestExampleConfigurationListsEverySetting(t *testing.T) {
	t.Parallel()

	written, err := fconfig.Example(fconfig.ExampleOptions{
		Header:    exampleHeader,
		EnvPrefix: "DIMMI",
		Path:      examplePath,
		Module:    "github.com/dimmihq/dimmi",
		Root:      repositoryRoot,
	}, &foundation.Config{}, &config.Config{})
	require.NoError(t, err)

	testkit.Golden(t, examplePath, written)
}

func TestExampleConfigurationLoads(t *testing.T) {
	t.Parallel()

	sources := fconfig.LoadOptions{Name: "dimmi", Files: []string{examplePath}}

	_, err := fconfig.Load[foundation.Config](t.Context(), sources)
	require.NoError(t, err)

	var app config.Config

	require.NoError(t, fconfig.LoadInto(t.Context(), sources, &app))
}
