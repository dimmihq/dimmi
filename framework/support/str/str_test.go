package str_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/support/str"
)

func TestStr(t *testing.T) {
	t.Parallel()

	t.Run("makes random letters and digits", func(t *testing.T) {
		t.Parallel()

		token := str.Random(64)

		assert.Len(t, token, 64)
		assert.Regexp(t, `^[0-9A-Za-z]{64}$`, token)
		assert.NotEqual(t, token, str.Random(64))
		assert.Len(t, str.Random(1), 1)
		assert.Empty(t, str.Random(0))
		assert.Empty(t, str.Random(-3))
	})

	t.Run("makes random digits for codes a person types", func(t *testing.T) {
		t.Parallel()

		code := str.RandomDigits(6)

		assert.Regexp(t, `^[0-9]{6}$`, code)
		assert.Regexp(t, `^[0-9]{400}$`, str.RandomDigits(400))
		assert.Empty(t, str.RandomDigits(0))
	})

	t.Run("makes url slugs", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "hello-world", str.Slug("Hello, World!"))
		assert.Equal(t, "creme-brulee", str.Slug("Crème Brûlée"))
	})

	t.Run("changes case", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "user_name", str.Snake("UserName"))
		assert.Equal(t, "user-name", str.Kebab("userName"))
		assert.Equal(t, "UserName", str.Studly("user_name"))
	})

	t.Run("pluralizes english words", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "people", str.Plural("person"))
		assert.Equal(t, "Categories", str.Plural("Category"))
		assert.Equal(t, "person", str.Singular("people"))
		assert.Equal(t, "invoice", str.Singular("invoices"))
	})

	t.Run("limits text by characters", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "The quick...", str.Limit("The quick brown fox", 10))
		assert.Equal(t, "héllo", str.Limit("héllo", 5))
		assert.Equal(t, "hé...", str.Limit("héllo", 2))
		assert.Equal(t, "...", str.Limit("héllo", -1))
	})

	t.Run("masks part of the text", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "************4242", str.Mask("4242424242424242", '*', 0, -4))
		assert.Equal(t, "tay***@example.com", str.Mask("taylor@example.com", '*', 3, 3))
		assert.Equal(t, "taylor@*******.com", str.Mask("taylor@example.com", '*', -11, 7))
		assert.Equal(t, "ab••", str.Mask("abcd", '•', 2, 0))
		assert.Equal(t, "••••", str.Mask("abcd", '•', -10, 0))
		assert.Equal(t, "abcd", str.Mask("abcd", '*', 9, 2))
		assert.Equal(t, "abcd", str.Mask("abcd", '*', 3, -2))
	})
}
