package email_test

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
fileLink finds a file's download link in an email's text.
*/
var fileLink = regexp.MustCompile(`https?://\S+/files/\S+`)

/*
storedFile keeps a file of the reply on the default disk, as an upload
would, and returns it as the channel hands it over: its size the one
given, its bytes a few.
*/
func storedFile(tc *tests.TestCase, r reply, name string, size int64) contracts.OutboundAttachment {
	tc.T.Helper()

	disk, err := tc.App.Storage.Default()
	require.NoError(tc.T, err)

	path := "tests/" + name
	require.NoError(tc.T, disk.PutStream(tc.T.Context(), path, bytes.NewReader([]byte("bytes of "+name)),
		filesystem.PutOptions{}))

	ws := r.conversation.WorkspaceID
	row := query(tc, ws, func(ctx context.Context, q *models.Queries) (models.Attachment, error) {
		return q.CreateAttachment(ctx, models.CreateAttachmentParams{
			WorkspaceID: r.conversation.WorkspaceID, MessageID: &r.message.ID, UploadedByType: "member",
			Disk: tc.App.Application.Config.Filesystems.Default, Path: path, Filename: name,
			ContentType: "application/pdf", SizeBytes: size, ScanStatus: "clean", Now: tc.Clock.Now(),
		})
	})

	return contracts.OutboundAttachment{
		ID: row.ID, Filename: name, ContentType: row.ContentType, SizeBytes: size,
		Open: func(ctx context.Context) (io.ReadCloser, error) { return disk.Open(ctx, path) },
	}
}

func TestLargeReplyFiles(t *testing.T) {
	t.Parallel()

	t.Run("attaches files that fit and links the rest, and the link downloads", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("files")
		r := memberReply(tc, startConversation(tc, address, customer), customer)
		r.outbound.Attachments = []contracts.OutboundAttachment{
			storedFile(tc, r, "receipt.pdf", 2<<10), storedFile(tc, r, "recording.pdf", 40<<20),
		}

		_, err := send(tc, r)
		require.NoError(t, err)

		mails := received(t, customer)
		require.Len(t, mails, 1)
		got := parsed(t, mails[0])

		require.Len(t, got.Attachments, 1)
		assert.Equal(t, "receipt.pdf", got.Attachments[0].Filename)
		assert.Contains(t, got.Text, "Some files were too large to attach")
		assert.Contains(t, got.Text, "recording.pdf (40.0 MB)")

		link := fileLink.FindString(got.Text)
		require.NotEmpty(t, link, got.Text)
		u, err := url.Parse(link)
		require.NoError(t, err)

		tc.Client().GetJSON(u.RequestURI()).AssertStatus(fhttp.StatusFound)

		q := u.Query()
		q.Set("signature", "00"+q.Get("signature")[2:])
		u.RawQuery = q.Encode()
		assert.NotEqual(t, fhttp.StatusFound, tc.Client().GetJSON(u.RequestURI()).Status)
	})
}
