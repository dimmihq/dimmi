package email_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/malware"

	"github.com/dimmihq/dimmi/app/models"
)

/*
picturesMail is an HTML email showing a picture in place by its content
id, the way mail clients send a pasted screenshot, with a PDF attached
beside it.
*/
func picturesMail(to string) []byte {
	return []byte("From: Dana <dana@customer.test>\r\nTo: " + to + "\r\nSubject: Broken checkout\r\n" +
		"Message-ID: <pictures-1@customer.test>\r\nDate: Mon, 2 Nov 2026 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=mixed\r\n\r\n" +
		"--mixed\r\nContent-Type: multipart/related; boundary=related\r\n\r\n" +
		"--related\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" +
		"<p>It breaks after <b>Pay</b>.</p><p><img src=\"cid:shot@customer.test\" alt=\"screenshot\"></p>\r\n" +
		"--related\r\nContent-Type: image/png\r\nContent-ID: <shot@customer.test>\r\n" +
		"Content-Disposition: attachment; filename=shot.png\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==\r\n" +
		"--related--\r\n--mixed\r\nContent-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=receipt.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		"JVBERi0xLjQK\r\n--mixed--\r\n")
}

func TestEmailedPictures(t *testing.T) {
	t.Parallel()

	t.Run("lets the team open an emailed file and names the picture the text shows", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, picturesMail(address.ForwardingAddress), "").AssertStatus(202)
		tc.Drain()

		found := conversations(tc)
		require.Len(t, found, 1)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, found[0].ID) + "/messages"
		timeline := tc.ActingAsOwner().GetJSON(path).AssertOK()

		assert.Contains(t, timeline.String("data.0.body"), "![screenshot](cid:shot@customer.test)")
		timeline.AssertJSONPath("data.0.attachments.#(filename==\"shot.png\").content_id", "shot@customer.test")
		assert.NotEmpty(t, timeline.String("data.0.attachments.#(filename==\"shot.png\").url"))
		assert.NotEmpty(t, timeline.String("data.0.attachments.#(filename==\"receipt.pdf\").url"))
	})

	t.Run("scans emailed files and blocks an infected one", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		tc.App.Scanner.Fake()
		address := defaultAddress(tc)
		mail := string(picturesMail(address.ForwardingAddress))
		mail = mail[:len(mail)-len("JVBERi0xLjQK\r\n--mixed--\r\n")] +
			base64.StdEncoding.EncodeToString([]byte(malware.EICAR())) + "\r\n--mixed--\r\n"

		deliver(tc, []byte(mail), "").AssertStatus(202)
		tc.Drain()

		found := conversations(tc)
		require.Len(t, found, 1)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, found[0].ID) + "/messages"
		timeline := tc.ActingAsOwner().GetJSON(path).AssertOK()

		tc.Table("attachments").AssertHas(map[string]any{"filename": "receipt.pdf", "scan_status": "infected"})
		tc.Table("attachments").AssertHas(map[string]any{"filename": "shot.png", "scan_status": "clean"})
		timeline.AssertJSONPath("data.0.attachments.#(filename==\"receipt.pdf\").blocked", true)
		assert.Empty(t, timeline.String("data.0.attachments.#(filename==\"receipt.pdf\").url"))
		assert.NotEmpty(t, timeline.String("data.0.attachments.#(filename==\"shot.png\").url"))
	})
}
