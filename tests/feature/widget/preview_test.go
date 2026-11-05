package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetSettingsPreview(t *testing.T) {
	t.Parallel()

	t.Run("returns the config with the change applied and saves nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		before := defaultWidget(t, tc)

		res := owner(tc).PostJSON(settingsPath(t, tc)+"/preview", testkit.JSON{
			"name":       "Docs",
			"appearance": testkit.JSON{"primary_color": "#5B5BD6", "position": "left"},
			"texts":      testkit.JSON{"greeting_intro": "Hello from the preview"},
		}).AssertOK()

		res.AssertJSONPath("data.app_id", appID(t, tc))
		res.AssertJSONPath("data.appearance.primary_color", "#5B5BD6")
		res.AssertJSONPath("data.appearance.position", "left")
		res.AssertJSONPath("data.texts.greeting_intro", "Hello from the preview")

		after := defaultWidget(t, tc)
		assert.Equal(t, before.Name, after.Name)
		assert.Equal(t, before.ConfigVersion, after.ConfigVersion)
		assert.JSONEq(t, string(before.Appearance), string(after.Appearance))
		owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().AssertJSONPath("data.name", before.Name)
	})

	t.Run("refuses a value the save would refuse", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).PostJSON(settingsPath(t, tc)+"/preview", testkit.JSON{
			"appearance": testkit.JSON{"primary_color": "red"},
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("appearance.primary_color")
	})

	t.Run("lets a member who can only read preview", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := memberClient(t, tc, enums.RoleMember)

		member.PostJSON(settingsPath(t, tc)+"/preview", testkit.JSON{"name": "Mine"}).AssertOK()
	})

	t.Run("finds no widget of another workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).PostJSON("/api/v1/widget-settings/wgt_00000000000000000000000000/preview", testkit.JSON{}).
			AssertNotFound()
	})
}
