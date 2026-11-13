package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
opener is a workflow that greets a visitor as they open a chat and ends
on one question with two buttons, which is all an opener may do.
*/
func opener() testkit.JSON {
	return testkit.JSON{
		"schema": 1, "trigger": "conversation.opener",
		"steps": []testkit.JSON{
			{"id": "s_hello01", "kind": "message", "settings": testkit.JSON{"text": "Hey there."}},
			{
				"id": "s_ask0001", "kind": "buttons", "settings": testkit.JSON{
					"text": "What brings you here?", "allow_typing": false,
				},
				"paths": []testkit.JSON{
					{"id": "p_billing", "label": "Billing", "steps": []testkit.JSON{
						{"id": "s_bill001", "kind": "message", "settings": testkit.JSON{"text": "Billing will reply."}},
					}},
					{"id": "p_setup01", "label": "Setting up", "steps": []testkit.JSON{
						{
							"id": "s_setup01", "kind": "message",
							"settings": testkit.JSON{"text": "Here is how to start."},
						},
					}},
				},
			},
		},
	}
}

func TestWidgetOpener(t *testing.T) {
	t.Parallel()

	t.Run("a workspace with no opener leaves the messenger to start chats", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().GetJSON(configPath(t, tc)).AssertOK().AssertJSONPath("data.features.opener", false)
	})

	t.Run("the messenger is told a workflow greets this workspace's visitors", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		publish(t, tc, opener())

		tc.Client().GetJSON(configPath(t, tc)).AssertOK().AssertJSONPath("data.features.opener", true)
	})

	t.Run("opening a chat greets the visitor before they write", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		publish(t, tc, opener())
		v := startVisitor(t, tc)

		conversation := v.client().PostJSON("/widget/v1/openers", testkit.JSON{}).
			AssertCreated().String("data.id")
		tc.Drain()

		res := v.client().GetJSON("/widget/v1/conversations/" + conversation + "/messages").AssertOK()
		res.AssertJSONPath("data.0.body", "Hey there.").
			AssertJSONPath("data.1.body", "What brings you here?").
			AssertJSONPath("data.1.choices.0.label", "Billing").
			AssertJSONPath("data.1.choices_only", true)
	})

	t.Run("the visitor's answer takes its path in the chat already open", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		publish(t, tc, opener())
		v := startVisitor(t, tc)

		conversation := v.client().PostJSON("/widget/v1/openers", testkit.JSON{}).
			AssertCreated().String("data.id")
		tc.Drain()

		v.client().PostJSON("/widget/v1/conversations/"+conversation+"/messages", testkit.JSON{
			"client_id": ids.New().String(), "body": "Setting up", "choice": "p_setup01",
		}).AssertCreated()
		tc.Drain()

		var page struct {
			Data []struct {
				Body *string `json:"body"`
			} `json:"data"`
		}
		v.client().GetJSON("/widget/v1/conversations/" + conversation + "/messages").AssertOK().Decode(&page)

		require.NotEmpty(t, page.Data)
		last := page.Data[len(page.Data)-1].Body
		if assert.NotNil(t, last) {
			assert.Equal(t, "Here is how to start.", *last)
		}
	})
}
