package contracts_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
TestAPIContract keeps the public REST API document in a golden file. The
SDKs are generated from that file and scripts/api-compat.ts compares it
with the last released API, so any change to a route, request or
resource shows up here first. Refresh it with make api-snapshot.
*/
func TestAPIContract(t *testing.T) {
	t.Parallel()

	t.Run("the REST API stays as the SDKs were generated against", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		doc, err := tc.App.Router.OpenAPI("/api/v1")
		require.NoError(t, err)

		var pretty bytes.Buffer
		require.NoError(t, json.Indent(&pretty, doc, "", "  "))

		testkit.Golden(t, "api_v1.openapi.json", append(pretty.Bytes(), '\n'))
		hintOnFailure(t)
	})
}
