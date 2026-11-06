package conversations_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
)

func TestReactions(t *testing.T) {
	t.Parallel()

	t.Run("adds a teammate's emoji once, counts each teammate, and takes it off", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		changed := record[appevents.MessageReactionsChanged](d.tc)
		bobClient, bob := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")
		note := d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Done?"}).AssertCreated().String("data.id")
		reactions := path + "/messages/" + note + "/reactions"

		d.owner.PostJSON(reactions, testkit.JSON{"emoji": "👍"}).AssertOK().
			AssertJSONPath("data.reactions.0.emoji", "👍").AssertJSONPath("data.reactions.0.count", 1)
		d.owner.PostJSON(reactions, testkit.JSON{"emoji": "👍"}).AssertOK().AssertJSONPath("data.reactions.0.count", 1)
		bobClient.PostJSON(reactions, testkit.JSON{"emoji": "👍"}).AssertOK().
			AssertJSONPath("data.reactions.0.count", 2).AssertJSONPath("data.reactions.0.member_ids.1", memberID(bob)).
			AssertJSONPath("data.author.name", d.tc.Owner.Name)

		d.owner.Send("DELETE", reactions+"/"+url.PathEscape("👍"), nil).AssertOK().
			AssertJSONPath("data.reactions.0.member_ids", []any{memberID(bob)})
		d.owner.Send("DELETE", reactions+"/"+url.PathEscape("👍"), nil).AssertOK()
		entries(d, path).AssertJSONPath("data.1.reactions.0.count", 1)

		d.tc.Drain()
		require.Len(t, changed.all(), 3)
	})

	t.Run("takes reactions on notes only, as emoji, up to twenty", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")
		reply := d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "On it"}).AssertCreated().String("data.id")
		note := d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Done?"}).AssertCreated().String("data.id")
		reactions := path + "/messages/" + note + "/reactions"

		d.owner.PostJSON(path+"/messages/"+reply+"/reactions", testkit.JSON{"emoji": "👍"}).
			AssertStatus(422).AssertJSONPath("code", "message_not_reactable")
		for _, bad := range []string{"ok", "1", "👍 👍", "a👍"} {
			d.owner.PostJSON(reactions, testkit.JSON{"emoji": bad}).AssertStatus(422)
		}

		for i := range 20 {
			emoji := string(rune(0x1F600 + i))
			d.owner.PostJSON(reactions, testkit.JSON{"emoji": emoji}).AssertOK()
		}

		d.owner.PostJSON(reactions, testkit.JSON{"emoji": "🎉"}).
			AssertStatus(422).AssertJSONPath("code", "reaction_limit_reached")
		d.owner.PostJSON(reactions, testkit.JSON{"emoji": string(rune(0x1F600))}).AssertOK().
			AssertJSONPath("data.reactions.#", 20)
		d.owner.PostJSON(path+"/messages/msg_nope/reactions", testkit.JSON{"emoji": "👍"}).AssertNotFound()
		open(t).owner.PostJSON(reactions, testkit.JSON{"emoji": "👍"}).AssertNotFound()
	})
}
