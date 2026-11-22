package str_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/support/str"
)

func TestSentences(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]int{
		"":     0,
		"One.": 1,
		"It costs $49.00 a month. Cancel any time.": 2,
		"Is it there? Yes! Good.":                   3,
		"تم الإلغاء. هل الرصيد مناسب؟":              2,
		"返金を申請しました。カードに戻しますか？":                      2,
	} {
		got := str.Sentences(text)
		assert.Len(t, got, want, text)
		assert.Equal(t, text, strings.Join(got, ""), "joining the sentences gives the text back")
	}
}
