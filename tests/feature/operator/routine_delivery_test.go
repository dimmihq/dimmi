package operator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/operatorslack"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
slackDestination makes a Slack channel destination in the team's room
on the installation, and returns its public id.
*/
func (s *slackDesk) slackDestination() string {
	s.t.Helper()
	var installation string
	require.NoError(s.t, s.d.TC.Operator().QueryRow(s.t.Context(),
		`SELECT id::text FROM integration_installations WHERE workspace_id = $1 LIMIT 1`,
		s.d.TC.Workspace.Workspace.ID).Scan(&installation))
	parsed, err := ids.Parse(installation)
	require.NoError(s.t, err)
	id := models.PublicID(models.PrefixIntegrationInstallation, parsed)
	return s.d.Owner.PostJSON("/api/v1/notification-destinations", testkit.JSON{
		"type": "slack_channel", "integration_id": id, "channel_id": teamRoom,
		"triggers": []string{"snooze_expired"},
	}).AssertCreated().String("data.id")
}

/*
ownerID is the owner's public member id.
*/
func (k *desk) ownerID() string {
	return models.PublicID(models.PrefixMember, k.d.TC.Workspace.Owner.ID)
}

func TestOperatorRoutineDelivery(t *testing.T) {
	t.Parallel()

	t.Run("tells the app, email and Slack what a run suggested", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		mail := s.d.TC.Mail()
		destination := s.slackDestination()
		conv := s.conversation("I want a refund")
		number := s.d.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK().String("data.number")
		routine := s.routine(testkit.JSON{
			"name": "Tagger", "prompt": "Tag refunds.",
			"delivery": testkit.JSON{
				"in_app_member_ids": []string{s.ownerID()}, "email_member_ids": []string{s.ownerID()},
				"slack_destination_ids": []string{destination},
			},
		})
		s.llm.Script(tools(call("propose_changes", tagging(t, number, "tag:"+number))),
			reply("- **Refunds:** one conversation needs the refund tag."))

		run := s.runRoutine(routine, false)
		s.d.TC.Drain()

		run = s.d.Owner.GetJSON("/api/v1/operator/routines/" + routine + "/runs/" + run.String("data.id")).AssertOK()
		run.AssertJSONPath("data.notified", true)
		var body struct {
			Data struct {
				DeliveryResults []map[string]any `json:"delivery_results"`
			} `json:"data"`
		}
		run.Decode(&body)
		results := body.Data.DeliveryResults
		require.Len(t, results, 3)
		for _, r := range results {
			assert.Equal(t, true, r["ok"], r)
		}

		bell := s.d.Owner.GetJSON("/api/v1/notifications").AssertOK()
		bell.AssertJSONPath("data.0.trigger", "operator_routine_finished")
		assert.Equal(t, "Tagger has suggestions for you", bell.String("data.0.title"))

		sent, _ := tests.Sent[appmail.OperatorRoutineMail](s.d.TC, mail)
		require.Len(t, sent, 1)
		assert.Equal(t, "findings", sent[0].Outcome)
		require.Len(t, sent[0].Suggestions, 1)
		assert.Equal(t, "Tag the refund", sent[0].Suggestions[0].Title)

		var posted []string
		for _, c := range s.slack.called("chat.postMessage") {
			if c.Form.Get("channel") == teamRoom {
				posted = append(posted, c.Form.Get("text"))
			}
		}
		require.Len(t, posted, 2)
		assert.Equal(t, "Tagger has suggestions for you", posted[0])
		assert.True(t, strings.HasPrefix(posted[1], "Proposed change: Tag the refund"), posted[1])

		changeset := s.d.Owner.GetJSON("/api/v1/operator/changesets?filter[routine]=" + routine).AssertOK().
			String("data.0.id")
		s.link()
		s.press(ownerSlack, operatorslack.ActionApply, changeset)
		s.d.Owner.GetJSON("/api/v1/operator/changesets?filter[routine]="+routine).AssertOK().
			AssertJSONPath("data.0.status", "applied")
	})

	t.Run("stays quiet when a run on findings found nothing", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		mail := k.d.TC.Mail()
		routine := k.routine(testkit.JSON{
			"name": "Watcher", "prompt": "Check.",
			"delivery": testkit.JSON{
				"in_app_member_ids": []string{k.ownerID()}, "email_member_ids": []string{k.ownerID()},
			},
		})
		k.llm.Script(tools(call("mark_noop", map[string]any{"reason": "Quiet week."})), reply("Quiet week."))

		k.runRoutine(routine, false).AssertJSONPath("data.outcome", "noop")
		k.d.TC.Drain()

		sent, _ := tests.Sent[appmail.OperatorRoutineMail](k.d.TC, mail)
		assert.Empty(t, sent)
		k.d.Owner.GetJSON("/api/v1/notifications").AssertOK().AssertJSONPath("data.#", float64(0))
	})

	t.Run("tells nobody about a dry run", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		mail := k.d.TC.Mail()
		routine := k.routine(testkit.JSON{
			"name": "Watcher", "prompt": "Check.", "notify_policy": "always",
			"delivery": testkit.JSON{"email_member_ids": []string{k.ownerID()}},
		})
		k.llm.Script(reply("All fine."))

		k.runRoutine(routine, true)
		k.d.TC.Drain()

		sent, _ := tests.Sent[appmail.OperatorRoutineMail](k.d.TC, mail)
		assert.Empty(t, sent)
	})

	t.Run("never tells a teammate who may not read what the routine reads", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		mail := k.d.TC.Mail()
		_, member := k.d.Join(t, enums.RoleMember)
		_, err := k.d.TC.Operator().Exec(k.t.Context(),
			`UPDATE workspace_members SET conversation_access = 'assigned' WHERE id = $1`, member.ID)
		require.NoError(t, err)
		routine := k.routine(testkit.JSON{
			"name": "Watcher", "prompt": "Check.", "notify_policy": "always",
			"delivery": testkit.JSON{"email_member_ids": []string{
				k.ownerID(), models.PublicID(models.PrefixMember, member.ID),
			}},
		})
		k.llm.Script(reply("All fine."))

		k.runRoutine(routine, false)
		k.d.TC.Drain()

		sent, _ := tests.Sent[appmail.OperatorRoutineMail](k.d.TC, mail)
		require.Len(t, sent, 1, "only the owner, who sees every conversation")
	})

	t.Run("refuses teammates of another workspace", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		k.d.Owner.PostJSON("/api/v1/operator/routines", testkit.JSON{
			"name": "Watcher", "prompt": "Check.",
			"delivery": testkit.JSON{"email_member_ids": []string{"mem_01m81tjwfhf8narqndekdnqzfk"}},
		}).AssertStatus(422).AssertJSONPath("code", "delivery_member_invalid")
	})
}
