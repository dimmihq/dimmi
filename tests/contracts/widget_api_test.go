package contracts_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetAPIContract(t *testing.T) {
	t.Parallel()

	t.Run("the messenger API stays as native and web clients were built against", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		doc, err := tc.App.Router.OpenAPI("/widget/v1")
		require.NoError(t, err)

		var pretty bytes.Buffer
		require.NoError(t, json.Indent(&pretty, doc, "", "  "))

		testkit.Golden(t, "widget_v1.openapi.json", append(pretty.Bytes(), '\n'))
	})
}
