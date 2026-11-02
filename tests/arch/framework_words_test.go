package arch_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit/arch"
)

/*
businessWords are the stems of the product's own vocabulary. The
framework is meant for any product, so none of them may appear in it.
Generic words every product has, such as tenant, user, member, channel
and message, are not on the list.
*/
func businessWords() []string {
	return []string{
		"workspace", "conversation", "contact", "inbox", "teammate", "widget", "helpcenter", "article",
		"ticket", "assignee", "snooze", "knowledge", "messenger", "intercom", "dimmi",
	}
}

func TestFrameworkWords(t *testing.T) {
	t.Parallel()

	t.Run("framework holds no business words", func(t *testing.T) {
		t.Parallel()

		arch.Load(t, module).Expect("framework/...").NotToContainWords(businessWords()...)
	})
}
