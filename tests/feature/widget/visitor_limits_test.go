package widget_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestVisitorLimits(t *testing.T) {
	t.Parallel()

	t.Run("stops a session that starts too many conversations", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		for range 5 {
			v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
				"client_id": ids.New().String(), "body": "Hello",
			}).AssertCreated()
		}

		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
			"client_id": ids.New().String(), "body": "Hello again",
		}).AssertStatus(429)
	})

	t.Run("stops a session that announces too many files", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		for range 10 {
			v.announce("notes.txt", "text/plain", 5).AssertCreated()
		}

		v.announce("notes.txt", "text/plain", 5).AssertStatus(429)
	})

	t.Run("keeps one session's count apart from another's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		busy, calm := startVisitor(t, tc), startVisitor(t, tc)

		for range 10 {
			busy.announce("notes.txt", "text/plain", 5).AssertCreated()
		}

		busy.announce("notes.txt", "text/plain", 5).AssertStatus(429)
		calm.announce("notes.txt", "text/plain", 5).AssertCreated()
	})
}
