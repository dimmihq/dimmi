package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

/*
orderByEmail is a read action that finds the customer's latest order by
their email, the only customer value it binds.
*/
func orderByEmail() testkit.JSON {
	return testkit.JSON{
		"name": "latest_order", "title": "Find latest order", "method": "GET",
		"description":   "Look up the customer's latest order.",
		"path_template": "/orders/latest", "query": testkit.JSON{"email": "{contact.email}"},
		"input_schema":    testkit.JSON{"type": "object", "properties": testkit.JSON{}},
		"response_fields": []testkit.JSON{{"name": "status", "path": "status"}},
		"channels":        []string{"widget"},
	}
}

func TestAiActionEmailCodes(t *testing.T) {
	t.Parallel()

	t.Run("an anonymous visitor confirms their email with a code before the action runs", func(t *testing.T) {
		t.Parallel()
		a := newActionDesk(t, widgetAgent)
		a.api.Stub("GET "+shopURL+"/orders/latest*", httpclient.Response{Body: []byte(`{"status":"shipped"}`)})
		a.action(a.connection(), orderByEmail())
		fake := a.d.TC.Mail()
		v := newVisitor(t, a.d)
		public, conv := v.start(t)
		a.llm.answer.Script(tools(call("latest_order", `{}`)), reply("Please confirm your email on the card."))
		a.run(conv, firstMessage(a, conv))

		c := a.onlyCall(conv)
		assert.Equal(t, "awaiting_verification", c.State)
		assert.Empty(t, a.sentTo("GET", "/api/orders/latest"))
		v.client().GetJSON("/widget/v1/conversations/" + public + "/action-calls/" + publicCall(c)).AssertOK()

		v.client().PostJSON("/widget/v1/email-verifications", testkit.JSON{"email": "Ada@Example.com"}).
			AssertStatus(fhttp.StatusAccepted).AssertJSONPath("data.email", "a***@example.com")
		mailed, _ := tests.LastSent[appmail.VisitorCodeMail](a.d.TC, fake)
		require.Len(t, mailed.Code, 6)
		wrong := "000000"
		if mailed.Code == wrong {
			wrong = "111111"
		}
		v.client().PostJSON("/widget/v1/email-verifications/checks", testkit.JSON{"code": wrong}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "email_code_invalid")
		v.client().PostJSON("/widget/v1/email-verifications/checks", testkit.JSON{"code": mailed.Code}).
			AssertOK().AssertJSONPath("data.email", "a***@example.com")

		a.llm.answer.Script(reply("Your latest order has shipped."))
		a.drain()
		sent := a.sentTo("GET", "/api/orders/latest")
		require.Len(t, sent, 1)
		assert.Equal(t, "ada@example.com", sent[0].URL.Query().Get("email"))
		assert.Equal(t, "succeeded", a.onlyCall(conv).State)
	})

	t.Run("burns a code after five wrong tries and throttles sending", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		v := newVisitor(t, d)
		v.client().PostJSON("/widget/v1/email-verifications/checks", testkit.JSON{"code": "123456"}).
			AssertStatus(fhttp.StatusGone).AssertJSONPath("code", "email_code_expired")
		v.client().PostJSON("/widget/v1/email-verifications", testkit.JSON{"email": "ada@example.com"}).
			AssertStatus(fhttp.StatusAccepted)
		v.client().PostJSON("/widget/v1/email-verifications", testkit.JSON{"email": "ada@example.com"}).
			AssertStatus(fhttp.StatusTooManyRequests)
		for range 5 {
			v.client().PostJSON("/widget/v1/email-verifications/checks", testkit.JSON{"code": "abcdef"}).
				AssertStatus(fhttp.StatusUnprocessableEntity)
		}
		v.client().PostJSON("/widget/v1/email-verifications/checks", testkit.JSON{"code": "abcdef"}).
			AssertStatus(fhttp.StatusTooManyRequests).AssertJSONPath("code", "email_code_locked")
	})
}
