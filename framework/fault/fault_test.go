package fault_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
)

func TestFault(t *testing.T) {
	t.Parallel()

	t.Run("describes itself", func(t *testing.T) {
		t.Parallel()
		err := fault.New(fault.Conflict, "member_exists", "That person is already a member.")

		assert.Equal(t, fault.Conflict, err.Kind())
		assert.Equal(t, "member_exists", err.Code())
		assert.Equal(t, "That person is already a member.", err.Message())
		assert.Equal(t, "member_exists: That person is already a member.", err.Error())
		assert.Nil(t, err.Meta())
	})

	t.Run("adds details without changing the original", func(t *testing.T) {
		t.Parallel()
		base := fault.New(fault.TooLarge, "file_too_large", "The file is too large.")

		limited := base.With("limit", 10).With("unit", "MB")

		assert.Nil(t, base.Meta())
		assert.Equal(t, map[string]any{"limit": 10, "unit": "MB"}, limited.Meta())
		assert.Equal(t, base.Error(), limited.Error())

		limited.Meta()["limit"] = 99
		assert.Equal(t, 10, limited.Meta()["limit"])
	})

	t.Run("matches copies of the same fault through wrapping", func(t *testing.T) {
		t.Parallel()
		declared := fault.New(fault.NotFound, "missing_member", "No such member.")
		wrapped := fmt.Errorf("remove member: %w", declared.With("id", 7))

		require.ErrorIs(t, wrapped, declared)
		require.NotErrorIs(t, wrapped, fault.New(fault.NotFound, "missing_tenant", "No such tenant."))
		require.NotErrorIs(t, wrapped, fault.New(fault.Gone, "missing_member", "No such member."))
		require.NotErrorIs(t, wrapped, errors.New("missing_member"))
	})

	t.Run("finds the kind and code in a chain", func(t *testing.T) {
		t.Parallel()
		wrapped := fmt.Errorf("invite: %w", fault.New(fault.RateLimited, "too_many_invites", "Slow down."))

		assert.Equal(t, fault.RateLimited, fault.KindOf(wrapped))
		assert.Equal(t, "too_many_invites", fault.CodeOf(wrapped))
	})

	t.Run("reports plain errors as internal without a code", func(t *testing.T) {
		t.Parallel()
		plain := errors.New("connection reset")

		assert.Equal(t, fault.Internal, fault.KindOf(plain))
		assert.Empty(t, fault.CodeOf(plain))
	})
}
