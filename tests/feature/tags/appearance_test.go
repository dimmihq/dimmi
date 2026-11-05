package tags_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestTagAppearance(t *testing.T) {
	t.Parallel()

	t.Run("gives a tag without a color the palette color its workspace uses least", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		colors := map[string]bool{}

		for _, name := range []string{"Billing", "Bug", "Feedback", "Question"} {
			color := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": name}).AssertCreated().String("data.color")
			assert.Regexp(t, `^#[0-9a-f]{6}$`, color)
			colors[color] = true
		}

		assert.Len(t, colors, 4)
	})

	t.Run("colors tags added to a conversation by name", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conversation := d.Customer(t, "ann", "Billing").String("data.id")

		path := "/api/v1/conversations/" + conversation + "/tags"
		d.Owner.PostJSON(path, testkit.JSON{"tags": []string{"vip", "refund"}}).AssertOK()

		var listed struct {
			Data []struct {
				Color *string `json:"color"`
			} `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/tags").AssertOK().Decode(&listed)
		assert.Len(t, listed.Data, 2)
		for _, tag := range listed.Data {
			assert.NotNil(t, tag.Color)
		}
	})

	t.Run("keeps a chosen icon, changes and clears it, and refuses one the app lacks", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Bug", "icon": "bug"}).AssertCreated().
			AssertJSONPath("data.icon", "bug")
		path := "/api/v1/tags/" + res.String("data.id")

		d.Owner.PatchJSON(path, testkit.JSON{"icon": "fire"}).AssertOK().AssertJSONPath("data.icon", "fire")
		d.Owner.PatchJSON(path, testkit.JSON{"icon": nil}).AssertOK().AssertJSONPath("data.icon", nil)
		d.Owner.PatchJSON(path, testkit.JSON{"icon": "unicorn"}).AssertStatus(422).
			AssertJSONPath("code", "tag_icon_unknown")
		d.Owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "Odd", "icon": "unicorn"}).AssertStatus(422)
	})
}
