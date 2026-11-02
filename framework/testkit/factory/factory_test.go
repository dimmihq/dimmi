package factory_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit/factory"
)

type user struct {
	ID    int
	Email string
	Admin bool
	Team  string
}

/*
users is the definition every subtest specializes.
*/
func users() *factory.Factory[user] {
	return factory.Define(func(seq int) user {
		return user{Email: "user" + strconv.Itoa(seq) + "@example.com", Team: "red"}
	})
}

func admin(u user) user {
	u.Admin = true

	return u
}

func team(name string) factory.State[user] {
	return func(u user) user {
		u.Team = name

		return u
	}
}

func TestFactory(t *testing.T) {
	t.Parallel()

	t.Run("makes distinct records from the definition", func(t *testing.T) {
		t.Parallel()
		f := users()

		made := f.Count(2).Make()
		one := f.MakeOne()

		assert.Equal(t, []user{
			{Email: "user1@example.com", Team: "red"},
			{Email: "user2@example.com", Team: "red"},
		}, made)
		assert.Equal(t, "user3@example.com", one.Email)
	})

	t.Run("applies states in order, then the sequence", func(t *testing.T) {
		t.Parallel()

		made := users().State(admin).State(team("blue")).Sequence(team("a"), team("b")).Count(3).Make()

		require.Len(t, made, 3)
		assert.True(t, made[0].Admin)
		assert.Equal(t, []string{"a", "b", "a"}, []string{made[0].Team, made[1].Team, made[2].Team})
	})

	t.Run("leaves the factory it was derived from unchanged", func(t *testing.T) {
		t.Parallel()
		base := users()

		base.State(admin).Count(5)

		assert.Len(t, base.Make(), 1)
		assert.False(t, base.MakeOne().Admin)
	})

	t.Run("stores records through the persist function", func(t *testing.T) {
		t.Parallel()
		stored := 0
		f := users().Persist(func(_ context.Context, u user) (user, error) {
			stored++
			u.ID = stored

			return u, nil
		})

		many, err := f.Count(2).Create(t.Context())
		require.NoError(t, err)
		one, err := f.CreateOne(t.Context())
		require.NoError(t, err)

		assert.Equal(t, []int{1, 2}, []int{many[0].ID, many[1].ID})
		assert.Equal(t, 3, one.ID)
	})

	t.Run("refuses to create without a persist function", func(t *testing.T) {
		t.Parallel()

		_, err := users().CreateOne(t.Context())

		assert.ErrorIs(t, err, factory.ErrNoPersist)
	})

	t.Run("stops at the first record it cannot store", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")

		_, err := users().Count(3).Persist(func(context.Context, user) (user, error) {
			return user{}, boom
		}).Create(t.Context())

		require.ErrorIs(t, err, boom)
		assert.Contains(t, err.Error(), "create record 1 of 3")
	})
}
