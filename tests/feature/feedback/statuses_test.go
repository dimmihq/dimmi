package feedback_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
order reads the workspace's status keys, first to last.
*/
func order(t *testing.T, client *testkit.Client) []string {
	t.Helper()
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	client.GetJSON("/api/v1/feedback-statuses").AssertOK().Decode(&out)
	keys := make([]string, len(out.Data))
	for i, s := range out.Data {
		keys[i] = s.ID
	}
	return keys
}

func TestFeedbackStatusOrder(t *testing.T) {
	t.Parallel()

	builtIn := []string{"in_progress", "planned", "under_review", "shipped", "closed"}
	chosen := []string{"under_review", "planned", "in_progress", "shipped", "closed"}

	t.Run("lists the built in order until the workspace chooses one", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		assert.Equal(t, builtIn, order(t, d.Owner))
	})

	t.Run("saves the whole order and lists it everywhere", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})

		b.d.Owner.PatchJSON("/api/v1/feedback-statuses", testkit.JSON{"statuses": chosen}).AssertOK().
			AssertJSONPath("data.0.id", "under_review").AssertJSONPath("data.0.name", "Under consideration")
		assert.Equal(t, chosen, order(t, b.d.Owner))

		page := string(b.page("/feedback").AssertOK().Body)
		review, planned, progress := strings.Index(page, `id="under-consideration-title"`),
			strings.Index(page, `id="planned-title"`), strings.Index(page, `id="in-progress-title"`)
		require.Positive(t, review)
		assert.Less(t, review, planned)
		assert.Less(t, planned, progress)
	})

	t.Run("refuses an order that misses, repeats or invents a status", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		for _, bad := range [][]string{
			{"under_review", "planned", "in_progress", "shipped"},
			{"under_review", "planned", "in_progress", "shipped", "shipped"},
			{"under_review", "planned", "in_progress", "shipped", "archived"},
		} {
			d.Owner.PatchJSON("/api/v1/feedback-statuses", testkit.JSON{"statuses": bad}).
				AssertStatus(422).AssertJSONPath("code", "statuses_invalid")
		}
		assert.Equal(t, builtIn, order(t, d.Owner))
	})

	t.Run("keeps each workspace's order to itself", func(t *testing.T) {
		t.Parallel()
		d, other := tests.NewDesk(t), tests.NewDesk(t)

		d.Owner.PatchJSON("/api/v1/feedback-statuses", testkit.JSON{"statuses": chosen}).AssertOK()

		assert.Equal(t, builtIn, order(t, other.Owner))
	})

	t.Run("lets members read the order but only settings managers change it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		user := d.TC.CreateUser()
		_, err := factories.Member(t.Context(), d.TC.App, factories.Membership{
			Workspace: d.TC.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		member := d.TC.In(user)

		assert.Equal(t, builtIn, order(t, member))
		member.PatchJSON("/api/v1/feedback-statuses", testkit.JSON{"statuses": chosen}).AssertForbidden()
		assert.Equal(t, builtIn, order(t, d.Owner))
	})
}
