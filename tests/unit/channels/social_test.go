package channels_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

func TestSocialDeliveries(t *testing.T) {
	t.Parallel()

	t.Run("reads a Messenger message with its files", func(t *testing.T) {
		t.Parallel()

		events, err := social.ParseDelivery([]byte(`{"object":"page","entry":[{"id":"100","time":1458692752478,
			"messaging":[{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":1458692752478,
			"message":{"mid":"m_1","text":"hello","attachments":[
				{"type":"image","payload":{"url":"https://scontent.xx.fbcdn.net/a.jpg"}},
				{"type":"sticker","payload":{"url":"https://scontent.xx.fbcdn.net/s.png",
					"sticker_id":369239263222822}},
				{"type":"fallback","payload":{"url":"https://example.test/post","title":"Tahiti"}},
				{"type":"fallback","payload":null}]}}]}]}`))
		require.NoError(t, err)
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, social.Facebook, event.Platform)
		assert.Equal(t, "100", event.Account)
		assert.Equal(t, "555", event.Customer)
		assert.Equal(t, time.UnixMilli(1458692752478).UTC(), event.At)
		assert.False(t, event.Standby)
		require.NotNil(t, event.Message)
		assert.Equal(t, "m_1", event.Message.ID)
		assert.Equal(t, "hello", event.Message.Text)

		files := event.Message.Attachments
		require.Len(t, files, 4)
		assert.True(t, files[0].File())
		assert.True(t, files[1].File())
		assert.True(t, files[2].Shared())
		assert.Equal(t, "Tahiti", files[2].Title())
		assert.False(t, files[3].File())
		assert.False(t, files[3].Shared())
		assert.Empty(t, files[3].Address())
	})

	t.Run("names the customer on an echo, whose parties are swapped", func(t *testing.T) {
		t.Parallel()

		events, err := social.ParseDelivery([]byte(`{"object":"page","entry":[{"id":"100","messaging":[
			{"sender":{"id":"100"},"recipient":{"id":"555"},"timestamp":1457764197627,
			"message":{"is_echo":true,"app_id":263902037430900,"mid":"m_2","text":"from the Page inbox"}}]}]}`))
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "555", events[0].Customer)
		assert.True(t, events[0].Message.Echo)
		assert.Equal(t, "263902037430900", string(events[0].Message.App))
	})

	t.Run("reads Instagram's story replies, mentions, unsends and unsupported messages", func(t *testing.T) {
		t.Parallel()

		events, err := social.ParseDelivery([]byte(`{"object":"instagram","entry":[{"id":"17841","time":1569262486134,
			"messaging":[
			{"sender":{"id":"900"},"recipient":{"id":"17841"},"timestamp":1569262485349,
			"message":{"mid":"ig_1","text":"love this",
				"reply_to":{"story":{"url":"https://lookaside.fbsbx.com/s","id":"777"}}}},
			{"sender":{"id":"900"},"recipient":{"id":"17841"},"timestamp":1569262485350,
			"message":{"mid":"ig_2","attachments":[{"type":"story_mention",
				"payload":{"url":"https://lookaside.fbsbx.com/m"}}]}},
			{"sender":{"id":"900"},"recipient":{"id":"17841"},"timestamp":1569262485351,
			 "message":{"mid":"ig_1","is_deleted":true}},
			{"sender":{"id":"900"},"recipient":{"id":"17841"},"timestamp":1569262485352,
			 "message":{"mid":"ig_3","is_unsupported":true}},
			{"sender":{"id":"900"},"recipient":{"id":"17841"},"timestamp":1569262485353,
			 "message":{"mid":"ig_4","attachments":[{"type":"ephemeral"}]}}]}]}`))
		require.NoError(t, err)
		require.Len(t, events, 5)

		assert.Equal(t, social.Instagram, events[0].Platform)
		require.NotNil(t, events[0].Message.ReplyTo.Story)
		assert.Equal(t, "777", string(events[0].Message.ReplyTo.Story.ID))
		assert.True(t, events[1].Message.Attachments[0].StoryMention())
		assert.False(t, events[1].Message.Attachments[0].File())
		assert.True(t, events[2].Message.Deleted)
		assert.True(t, events[3].Message.Unsupported)
		assert.False(t, events[4].Message.Attachments[0].File())
	})

	t.Run("reads reactions, reads, edits, postbacks and handovers", func(t *testing.T) {
		t.Parallel()

		events, err := social.ParseDelivery([]byte(`{"object":"page","entry":[{"id":"100","messaging":[
			{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":1,
				"reaction":{"mid":"m_1","action":"react","reaction":"love","emoji":"❤"}},
			{"sender":{"id":"555"},"recipient":{"id":"100"},
				"timestamp":2,"reaction":{"mid":"m_1","action":"unreact"}},
			{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":3,"read":{"watermark":1458668856253}},
			{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":4,
				"message_edit":{"mid":"m_1","text":"hello!","num_edit":1}},
			{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":"1527459824",
				"postback":{"mid":"m_5","title":"Get started","payload":"x"}},
			{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":6,
				"pass_thread_control":{"previous_owner_app_id":"263902037430900","new_owner_app_id":123}},
			{"sender":{"id":"555"},"recipient":{"id":"100"},"timestamp":7,
				"take_thread_control":{"previous_owner_app_id":123,"new_owner_app_id":"263902037430900"}}]}]}`))
		require.NoError(t, err)
		require.Len(t, events, 7)

		assert.Equal(t, "❤", events[0].Reaction.Emoji)
		assert.False(t, events[0].Reaction.Removed())
		assert.True(t, events[1].Reaction.Removed())
		assert.Equal(t, "1458668856253", string(events[2].Read.Watermark))
		assert.Equal(t, "hello!", events[3].Edit.Text)
		assert.Equal(t, "Get started", events[4].Postback.Title)
		assert.Equal(t, time.Unix(1527459824, 0).UTC(), events[4].At)
		assert.Equal(t, "123", string(events[5].Control.Owner))
		assert.Equal(t, "263902037430900", string(events[6].Control.Owner))
	})

	t.Run("reads standby events and the changes envelope", func(t *testing.T) {
		t.Parallel()

		events, err := social.ParseDelivery([]byte(`{"object":"instagram","entry":[{"id":"17841","time":1569262486134,
			"standby":[{"sender":{"id":"900"},"recipient":{"id":"17841"},"message":{"mid":"ig_9","text":"watching"}}],
			"changes":[
				{"field":"messages","value":{"sender":{"id":"901"},"recipient":{"id":"17841"},
					"timestamp":1569262485349,"message":{"mid":"ig_10","text":"hi"}}},
				{"field":"comments","value":{"id":"1","text":"a comment"}}]}]}`))
		require.NoError(t, err)
		require.Len(t, events, 2)
		assert.True(t, events[0].Standby)
		assert.Equal(t, time.UnixMilli(1569262486134).UTC(), events[0].At)
		assert.Equal(t, "901", events[1].Customer)
		assert.False(t, events[1].Standby)
	})

	t.Run("refuses what is not a delivery", func(t *testing.T) {
		t.Parallel()

		_, err := social.ParseDelivery([]byte(`{"object":"whatsapp_business_account","entry":[]}`))
		require.ErrorIs(t, err, social.ErrNotDelivery)

		_, err = social.ParseDelivery([]byte(`not json`))
		require.ErrorIs(t, err, social.ErrNotDelivery)
	})
}

func TestSocialWindow(t *testing.T) {
	t.Parallel()

	driver := &social.Driver{Platform: social.Instagram, Standard: 24 * time.Hour, Human: 7 * 24 * time.Hour}
	wrote := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	window := social.Window{LastInboundAt: wrote}

	assert.Equal(t, social.PhaseOpen, driver.PhaseAt(window, wrote.Add(23*time.Hour)))
	assert.Equal(t, social.PhaseHuman, driver.PhaseAt(window, wrote.Add(24*time.Hour)))
	assert.Equal(t, social.PhaseHuman, driver.PhaseAt(window, wrote.Add(7*24*time.Hour-time.Minute)))
	assert.Equal(t, social.PhaseClosed, driver.PhaseAt(window, wrote.Add(7*24*time.Hour)))
	assert.Equal(t, social.PhaseClosed, driver.PhaseAt(social.Window{}, wrote))

	assert.Equal(t, "instagram", driver.Type())
	assert.Equal(t, enums.IdentityTypeInstagramUser, social.Instagram.Identity())
	assert.Equal(t, enums.IdentityTypeFacebookUser, social.Facebook.Identity())
	assert.Equal(t, 1000, driver.Capabilities(models.ChannelConnection{}).MaxBodyChars)
	assert.Equal(t, contracts.StartNever, driver.Capabilities(models.ChannelConnection{}).StartConversation)
}

func TestSocialThreadKeys(t *testing.T) {
	t.Parallel()

	target, ok := social.ParseThreadKey(social.ThreadKey("555"))
	assert.True(t, ok)
	assert.Equal(t, "555", target.Customer)

	_, ok = social.ParseThreadKey("555")
	assert.False(t, ok)

	_, ok = social.ParseThreadKey("dm:")
	assert.False(t, ok)
}

func TestSocialErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		refused  meta.GraphError
		code     string
		sentinel error
	}{
		{
			"outside the window on Instagram",
			meta.GraphError{Code: 10, Subcode: 2534022},
			social.CodeWindowClosed, contracts.ErrWindowClosed,
		},
		{
			"outside the window on Messenger",
			meta.GraphError{Code: 10, Subcode: 2018278},
			social.CodeWindowClosed, contracts.ErrWindowClosed,
		},
		{
			"another app answers the Page thread",
			meta.GraphError{Code: 10, Subcode: 2018300},
			social.CodeNotOwner, contracts.ErrPermanent,
		},
		{
			"another app answers the Instagram thread",
			meta.GraphError{Code: 100, Subcode: 2534037},
			social.CodeNotOwner, contracts.ErrPermanent,
		},
		{
			"message access is off",
			meta.GraphError{Code: 200, Subcode: 2534041},
			social.CodeAccessOff, contracts.ErrConnectionBroken,
		},
		{
			"the person blocked the account",
			meta.GraphError{Code: 551},
			social.CodeUnreachable, contracts.ErrRecipientUnreachable,
		},
		{
			"the file is too big",
			meta.GraphError{Code: 100, Subcode: 2018109},
			social.CodeContentRejected, contracts.ErrContentRejected,
		},
		{
			"the token died",
			meta.GraphError{Code: 190, Subcode: 460},
			social.CodeConnectionBroken, contracts.ErrConnectionBroken,
		},
		{
			"a permission is missing",
			meta.GraphError{Code: 10},
			social.CodeConnectionBroken, contracts.ErrConnectionBroken,
		},
		{"the account is restricted", meta.GraphError{Code: 368}, social.CodeRestricted, contracts.ErrPermanent},
		{"a bad parameter", meta.GraphError{Code: 100}, social.CodeRefused, contracts.ErrPermanent},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			failed, ok := errors.AsType[*contracts.SendError](social.MapError(&c.refused))
			require.True(t, ok)
			assert.Equal(t, c.code, failed.Code)
			require.ErrorIs(t, failed, c.sentinel)
			assert.NotEmpty(t, failed.Message)
		})
	}

	t.Run("waits when Meta asks to slow down, and tries again when Meta is down", func(t *testing.T) {
		t.Parallel()

		limited, ok := errors.AsType[*contracts.RateLimitedError](social.MapError(&meta.GraphError{Code: 613}))
		require.True(t, ok)
		assert.Equal(t, time.Minute, limited.RetryAfter)

		down, ok := errors.AsType[*contracts.SendError](social.MapError(&meta.GraphError{Code: 2}))
		require.True(t, ok)
		assert.Equal(t, social.CodeUnavailable, down.Code)
		require.NoError(t, down.Err)

		tagged, ok := errors.AsType[*contracts.SendError](social.MapError(&meta.GraphError{
			Code:    613,
			Subcode: 2018338,
		}))
		require.True(t, ok)
		assert.Equal(t, social.CodeRefused, tagged.Code)

		assert.Equal(t, assert.AnError, social.MapError(assert.AnError))
	})
}
