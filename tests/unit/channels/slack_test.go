package channels_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/integrations/chatspace"
	"github.com/dimmihq/dimmi/lang"
)

func TestSlackFormat(t *testing.T) {
	t.Parallel()

	t.Run("escapes markup so nothing can ping a channel", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "&lt;!here&gt; &amp; &lt;@U1&gt;", slackdriver.ToSlack("<!here> & <@U1>"))
	})

	t.Run("turns tables into code blocks and pictures into links", func(t *testing.T) {
		t.Parallel()
		out := slackdriver.ToSlack("Look:\n| a | b |\n|---|---|\n![chart](https://x.test/c.png)\n```\n![x](y)\n```")
		assert.Equal(t, "Look:\n```\n| a | b |\n|---|---|\n```\n[chart](https://x.test/c.png)\n```\n![x](y)\n```", out)
		assert.Equal(t, "```\n| a |\n```", slackdriver.ToSlack("| a |"))
	})

	t.Run("truncates long text with an ellipsis", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "abc", slackdriver.Truncate("abc", 3))
		assert.Equal(t, "ab…", slackdriver.Truncate("abcd", 3))
	})

	t.Run("reads rich text blocks into Markdown", func(t *testing.T) {
		t.Parallel()
		blocks := json.RawMessage(`[{"type":"rich_text","elements":[
			{"type":"rich_text_section","elements":[
				{"type":"text","text":"Hi ","style":{}},{"type":"text","text":"bold","style":{"bold":true}},
				{"type":"text","text":" "},{"type":"text","text":"x","style":{"code":true}},
				{"type":"text","text":" "},{"type":"text","text":"gone","style":{"italic":true,"strike":true}},
				{"type":"link","url":"https://a.test","text":"site"},{"type":"link","url":"javascript:alert(1)"},
				{"type":"user","user_id":"U1"},{"type":"channel","channel_id":"C1"},
				{"type":"emoji","name":"tada"},{"type":"broadcast","range":"here"}]},
			{"type":"rich_text_list","style":"ordered","elements":[
				{"type":"rich_text_section","elements":[{"type":"text","text":"one"}]},
				{"type":"rich_text_section","elements":[{"type":"text","text":"two"}]}]},
			{"type":"rich_text_list","style":"bullet","indent":1,"elements":[
				{"type":"rich_text_section","elements":[{"type":"text","text":"dot"}]}]},
			{"type":"rich_text_quote","elements":[{"type":"text","text":"quoted\nmore"}]},
			{"type":"rich_text_preformatted","elements":[{"type":"text","text":"code"},{"type":"link","url":"https://u.test"}]}
		]},{"type":"divider"}]`)

		out := slackdriver.ToMarkdown("ignored", blocks, func(id string) string {
			if id == "U1" {
				return "Ann"
			}

			return ""
		})
		assert.Equal(t, "Hi **bold** `x` *~~gone~~*[site](https://a.test)javascript:alert(1)@Ann#C1:tada:@here\n\n"+
			"1. one\n2. two\n\n  - dot\n\n> quoted\n> more\n\n```\ncodehttps://u.test\n```", out)
	})

	t.Run("falls back to mrkdwn text", func(t *testing.T) {
		t.Parallel()
		out := slackdriver.ToMarkdown("*b* _i_ ~s~ <https://a.test|site> <https://b.test> <@U2> <#C1|general> "+
			"<#C2> <!here> &amp;", nil, nil)
		assert.Equal(t, "**b** *i* ~~s~~ [site](https://a.test) https://b.test @U2 #general #C2 @here &", out)
	})
}

func TestSlackKeys(t *testing.T) {
	t.Parallel()

	t.Run("reads thread keys of rooms and direct messages back", func(t *testing.T) {
		t.Parallel()
		room := ids.New()

		thread, ok := slackdriver.ParseThreadKey(slackdriver.RoomThreadKey(room, "1.2"))
		require.True(t, ok)
		assert.Equal(t, room, thread.RoomID)
		assert.Equal(t, "1.2", thread.RootTS)
		assert.False(t, thread.Direct())

		thread, ok = slackdriver.ParseThreadKey(slackdriver.DirectThreadKey("D1"))
		require.True(t, ok)
		assert.True(t, thread.Direct())

		for _, bad := range []string{"email:x", "slack:r:nope:1", "slack:r:" + room.String(), "slack:d:"} {
			_, ok := slackdriver.ParseThreadKey(bad)
			assert.False(t, ok, bad)
		}
	})

	t.Run("reads message ids and control ids back", func(t *testing.T) {
		t.Parallel()
		channel, ts, ok := slackdriver.SplitMessageID(slackdriver.MessageID("C1", "1.2"))
		assert.True(t, ok)
		assert.Equal(t, []string{"C1", "1.2"}, []string{channel, ts})
		_, _, ok = slackdriver.SplitMessageID("nope")
		assert.False(t, ok)

		msg, control, ok := slackdriver.ParseControlActionID(slackdriver.ControlActionID("msg_1", "solved"))
		assert.True(t, ok)
		assert.Equal(t, []string{"msg_1", "solved"}, []string{msg, control})

		for _, bad := range []string{"x:msg:solved", "c:", "c:msg:", "c::solved"} {
			_, _, ok := slackdriver.ParseControlActionID(bad)
			assert.False(t, ok, bad)
		}

		assert.Equal(t, "file:F1", slackdriver.FileID("F1"))
		assert.Equal(t, "T1:U1", slackdriver.UserIdentity("T1", "U1"))
	})
}

func TestSlackSignature(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_000, 0)
	body := []byte(`{"type":"event_callback"}`)
	ts := "1800000000"

	t.Run("accepts Slack's signature within the window and nothing else", func(t *testing.T) {
		t.Parallel()
		signature := slackdriver.SignatureHeader("s", ts, body)
		good := slackdriver.SignedRequest{Timestamp: ts, Signature: signature, Body: body}
		require.NoError(t, slackdriver.VerifySignature("s", good, now))

		later := now.Add(10 * time.Minute)
		require.ErrorIs(t, slackdriver.VerifySignature("s", good, later), slackdriver.ErrSignatureStale)
		require.ErrorIs(t, slackdriver.VerifySignature("other", good, now), slackdriver.ErrSignatureInvalid)
		require.ErrorIs(t, slackdriver.VerifySignature("", good, now), slackdriver.ErrSignatureInvalid)

		bad := good
		bad.Timestamp = "soon"
		require.ErrorIs(t, slackdriver.VerifySignature("s", bad, now), slackdriver.ErrSignatureStale)

		bad = good
		bad.Signature = "v0=zz"
		require.ErrorIs(t, slackdriver.VerifySignature("s", bad, now), slackdriver.ErrSignatureInvalid)

		bad.Signature = "v1=00"
		require.ErrorIs(t, slackdriver.VerifySignature("s", bad, now), slackdriver.ErrSignatureInvalid)
	})
}

func TestSlackErrors(t *testing.T) {
	t.Parallel()

	t.Run("leaves errors that are not Slack's answers transient", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, slackdriver.MapError("x", nil))

		transient := slackdriver.MapError("x", errors.New("connection reset"))
		assert.NotErrorIs(t, transient, contracts.ErrPermanent)
		assert.Empty(t, slackdriver.Code(transient))
	})
}

func TestSlackManifest(t *testing.T) {
	t.Parallel()

	t.Run("leaves request addresses out for a socket", func(t *testing.T) {
		t.Parallel()
		web := slackdriver.Manifest(slackdriver.ManifestOptions{AppURL: "https://d.test", ConnectionID: "chn_1"})
		socket := slackdriver.Manifest(slackdriver.ManifestOptions{Socket: true, BotScopes: []string{"chat:write"}})

		raw, _ := json.Marshal(web)
		assert.Contains(t, string(raw), "https://d.test/webhooks/slack/events/chn_1")
		assert.Contains(t, string(raw), `"name":"dimmi Support"`)

		raw, _ = json.Marshal(socket)
		assert.NotContains(t, string(raw), "request_url")
		assert.Contains(t, string(raw), `"bot":["chat:write"]`)

		link, err := slackdriver.CreateAppURL(web)
		require.NoError(t, err)
		assert.Contains(t, link, "manifest_json=")
	})
}

func TestPleasantries(t *testing.T) {
	t.Parallel()

	p, err := chatspace.LoadPleasantries(lang.Files(), 40)
	require.NoError(t, err)

	for _, text := range []string{
		"thanks!", "Thank you so much 🙏", "ok, got it", ":thumbsup:", "👍", "merci beaucoup",
		"lol thanks", "ありがとう",
	} {
		assert.True(t, p.Is(text), text)
	}

	for _, text := range []string{
		"thanks, but it still fails?", "", "the export fails",
		"thanks ok thanks ok thanks ok thanks ok thanks ok thanks",
	} {
		assert.False(t, p.Is(text), text)
	}
}
