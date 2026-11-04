package contracts_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/api"
	"github.com/dimmihq/dimmi/app/drivers/channel/fake"
	"github.com/dimmihq/dimmi/app/models"
)

func TestChannelDrivers(t *testing.T) {
	t.Parallel()

	for _, driver := range []contracts.Channel{api.Driver{}, &fake.Driver{}} {
		t.Run("keeps the contract: "+driver.Type(), func(t *testing.T) {
			t.Parallel()
			conn := models.ChannelConnection{Type: driver.Type()}
			caps := driver.Capabilities(conn)

			assert.Contains(t,
				[]contracts.Transport{contracts.TransportPush, contracts.TransportRealtime, contracts.TransportNone},
				caps.Transport)
			assert.NotEmpty(t, caps.StartConversation)

			result, err := driver.Send(t.Context(), conn, contracts.OutboundMessage{IdempotencyKey: "msg_1"})
			require.NoError(t, err)

			if caps.Transport == contracts.TransportNone {
				assert.Empty(t, result.ChannelMessageID)

				return
			}

			assert.NotEmpty(t, result.ChannelMessageID)
		})
	}

	t.Run("fails the sends a test scripts, in order, and records the rest", func(t *testing.T) {
		t.Parallel()
		driver := &fake.Driver{}
		driver.FailNext(contracts.ErrPermanent)

		_, err := driver.Send(t.Context(), models.ChannelConnection{}, contracts.OutboundMessage{})
		require.ErrorIs(t, err, contracts.ErrPermanent)

		_, err = driver.Send(t.Context(), models.ChannelConnection{}, contracts.OutboundMessage{BodyMarkdown: "hi"})
		require.NoError(t, err)
		require.Len(t, driver.Sent(), 1)
		assert.Equal(t, "hi", driver.Sent()[0].BodyMarkdown)
	})
}

func TestChannelManager(t *testing.T) {
	t.Parallel()

	t.Run("finds drivers by type and refuses a second one", func(t *testing.T) {
		t.Parallel()
		m := channels.NewManager(false)
		require.NoError(t, m.Register(api.Driver{}))

		driver, err := m.Driver(api.Type)
		require.NoError(t, err)
		assert.Equal(t, api.Type, driver.Type())

		require.ErrorIs(t, m.Register(api.Driver{}), channels.ErrDuplicateDriver)

		_, err = m.Driver("telegram")
		require.ErrorIs(t, err, channels.ErrUnknownChannel)
	})

	t.Run("hands out one fake only where fakes are allowed", func(t *testing.T) {
		t.Parallel()

		_, err := channels.NewManager(false).Fake()
		require.True(t, errors.Is(err, channels.ErrFakesNotAllowed))

		m := channels.NewManager(true)
		first, err := m.Fake()
		require.NoError(t, err)

		second, err := m.Fake()
		require.NoError(t, err)
		assert.Same(t, first, second)

		driver, err := m.Driver(fake.Type)
		require.NoError(t, err)
		assert.Same(t, first, driver)
	})
}
