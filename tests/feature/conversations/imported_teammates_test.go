package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/importedteammates"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestImportedTeammatesInPickers(t *testing.T) {
	t.Parallel()

	t.Run("a teammate waiting to be enabled is never offered, mentioned or assigned", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		teammate, err := d.tc.App.Actions.CreateImportedTeammate.Handle(t.Context(),
			importedteammates.CreateImportedTeammateInput{
				WorkspaceID: d.tc.Workspace.Workspace.ID, Email: "zainab@example.com", Name: "Zainab Bello",
				Role: enums.RoleAdmin, Source: "intercom",
			})
		require.NoError(t, err)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")

		d.owner.GetJSON(path+"/mentionables?q=Zainab").AssertOK().AssertJSONPath("data.members", []any{})
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "<@" + memberID(teammate.Member) + ">"}).AssertCreated()
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": memberID(teammate.Member)}).
			AssertStatus(422).AssertJSONPath("code", "member_inactive")

		d.tc.Table("message_mentions").AssertCount(0, nil)
	})
}
