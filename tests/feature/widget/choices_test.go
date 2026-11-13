package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
menu is a first-message workflow that asks one question with two buttons,
each answering in its own words.
*/
func menu() testkit.JSON {
	return testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{{
			"id": "s_ask0001", "kind": "buttons", "settings": testkit.JSON{"text": "What do you need help with?"},
			"paths": []testkit.JSON{
				{"id": "p_billing", "label": "Billing", "steps": []testkit.JSON{
					{"id": "s_bill001", "kind": "message", "settings": testkit.JSON{"text": "Billing will reply."}},
				}},
				{"id": "p_setup01", "label": "Setting up", "steps": []testkit.JSON{
					{"id": "s_setup01", "kind": "message", "settings": testkit.JSON{"text": "Here is how to start."}},
				}},
			},
		}},
	}
}

/*
publish puts the definition live in the workspace.
*/
func publish(t *testing.T, tc *tests.TestCase, spec testkit.JSON) {
	t.Helper()

	id := owner(tc).PostJSON("/api/v1/workflows", testkit.JSON{"name": "Menu", "spec": spec}).
		AssertCreated().String("data.id")
	owner(tc).PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).AssertOK()
}

func TestWidgetChoices(t *testing.T) {
	t.Parallel()

	t.Run("a question reaches the messenger as buttons, not as a numbered list", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		publish(t, tc, menu())
		v := startVisitor(t, tc)

		conversation := v.startConversation(t, "hello there")
		tc.Drain()

		res := v.client().GetJSON("/widget/v1/conversations/" + conversation + "/messages").AssertOK()
		question := res.String("data.1.body")
		assert.Equal(t, "What do you need help with?", question)
		res.AssertJSONPath("data.1.choices.0.id", "p_billing").
			AssertJSONPath("data.1.choices.0.label", "Billing").
			AssertJSONPath("data.1.choices.1.id", "p_setup01").
			AssertJSONPath("data.1.choices.1.label", "Setting up")
	})

	t.Run("a tapped button takes its path, whatever the customer's message reads", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		publish(t, tc, menu())
		v := startVisitor(t, tc)

		conversation := v.startConversation(t, "hello there")
		tc.Drain()

		// The body matches no label, so only the path can decide the branch.
		v.client().PostJSON("/widget/v1/conversations/"+conversation+"/messages", testkit.JSON{
			"client_id": ids.New().String(), "body": "nothing like a label", "choice": "p_setup01",
		}).AssertCreated()
		tc.Drain()

		var page struct {
			Data []struct {
				Body *string `json:"body"`
			} `json:"data"`
		}
		v.client().GetJSON("/widget/v1/conversations/" + conversation + "/messages").AssertOK().Decode(&page)

		last := page.Data[len(page.Data)-1].Body
		if assert.NotNil(t, last) {
			assert.Equal(t, "Here is how to start.", *last)
		}
	})

	t.Run("a typed answer still matches a label, for a customer who writes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		publish(t, tc, menu())
		v := startVisitor(t, tc)

		conversation := v.startConversation(t, "hello there")
		tc.Drain()

		v.client().PostJSON("/widget/v1/conversations/"+conversation+"/messages", testkit.JSON{
			"client_id": ids.New().String(), "body": "billing",
		}).AssertCreated()
		tc.Drain()

		var page struct {
			Data []struct {
				Body *string `json:"body"`
			} `json:"data"`
		}
		v.client().GetJSON("/widget/v1/conversations/" + conversation + "/messages").AssertOK().Decode(&page)

		last := page.Data[len(page.Data)-1].Body
		if assert.NotNil(t, last) {
			assert.Equal(t, "Billing will reply.", *last)
		}
	})
}
