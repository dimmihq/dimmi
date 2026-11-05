package widget_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
pngOf encodes a picture of the size.
*/
func pngOf(t *testing.T, width, height int) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))))

	return buf.Bytes()
}

/*
hugePNG is a small file whose header claims a picture of 10000 by 5000
pixels, the shape of a decompression bomb.
*/
func hugePNG(t *testing.T) []byte {
	t.Helper()

	data := pngOf(t, 1, 1)
	binary.BigEndian.PutUint32(data[16:20], 10000)
	binary.BigEndian.PutUint32(data[20:24], 5000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))

	return data
}

/*
announce has the visitor start an upload of the file.
*/
func (v *visitor) announce(name, contentType string, size int) *testkit.Response {
	return v.client().PostJSON("/widget/v1/attachments", testkit.JSON{
		"filename": name, "content_type": contentType, "size_bytes": size,
	})
}

/*
upload has the visitor announce the file and send its bytes to the
server, returning the upload's id.
*/
func (v *visitor) upload(t *testing.T, name, contentType string, content []byte) string {
	t.Helper()

	id := v.announce(name, contentType, len(content)).AssertCreated().String("data.id")
	v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", content).AssertNoContent()

	return id
}

/*
complete has the visitor say the upload's bytes arrived.
*/
func (v *visitor) complete(id string) *testkit.Response {
	return v.client().PatchJSON("/widget/v1/attachments/"+id, testkit.JSON{"status": "uploaded"})
}

func TestWidgetAttachments(t *testing.T) {
	t.Parallel()

	t.Run("uploads a picture to the server and sends it with a message", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		picture := pngOf(t, 4, 3)

		res := v.announce("error.png", "image/png", len(picture)).AssertCreated()
		res.AssertJSONPath("data.object", "attachment_upload").AssertJSONPath("data.status", "pending")
		res.AssertJSONPath("data.upload.method", "PUT")
		res.AssertJSONPath("data.upload.headers.Content-Type", "image/png")
		id := res.String("data.id")
		assert.True(t, strings.HasPrefix(id, "att_"))
		assert.True(t, strings.HasSuffix(res.String("data.upload.url"), "/widget/v1/attachments/"+id+"/content"))

		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", picture).AssertNoContent()

		done := v.complete(id).AssertOK()
		done.AssertJSONMissing("data.blocked")
		done.AssertJSONPath("data.object", "attachment").AssertJSONPath("data.width", 4.0)
		done.AssertJSONPath("data.height", 3.0).AssertJSONPath("data.content_type", "image/png")

		sent := v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
			"client_id": ids.New().String(), "body": "", "attachment_ids": []string{id},
		}).AssertCreated()
		assert.Equal(t, id, sent.String("meta.message.attachments.0.id"))

		tc.Table("attachments").AssertHas(map[string]any{"filename": "error.png", "scan_status": "skipped"})
	})

	t.Run("completing twice answers the same upload", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.upload(t, "notes.txt", "text/plain", []byte("hello there"))

		first := v.complete(id).AssertOK()
		second := v.complete(id).AssertOK()
		assert.Equal(t, first.String("data.id"), second.String("data.id"))
	})

	t.Run("refuses a file over the widget's limit", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.announce("big.pdf", "application/pdf", 11<<20).AssertStatus(fhttp.StatusRequestEntityTooLarge).
			AssertJSONPath("code", "attachment_too_large")
	})

	t.Run("refuses a program by its name or its type", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.announce("setup.exe", "application/octet-stream", 10).AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "attachment_type_blocked")
		v.announce("page.txt", "text/html", 10).AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "attachment_type_blocked")
	})

	t.Run("removes a program renamed as a picture", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.upload(t, "cat.png", "image/png", append([]byte("MZ"), make([]byte, 64)...))

		v.complete(id).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "attachment_type_blocked")
		tc.Table("attachments").AssertMissing(map[string]any{"filename": "cat.png"})
	})

	t.Run("removes a file of another size than announced", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 10).AssertCreated().String("data.id")
		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("short")).AssertNoContent()

		v.complete(id).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "attachment_size_mismatch")
		tc.Table("attachments").AssertMissing(map[string]any{"filename": "a.txt"})
	})

	t.Run("stops a body larger than announced", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 5).AssertCreated().String("data.id")

		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("far too long")).
			AssertStatus(fhttp.StatusRequestEntityTooLarge).AssertJSONPath("code", "attachment_too_large")
		v.complete(id).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "attachment_not_uploaded")
	})

	t.Run("refuses a picture with too many pixels", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.upload(t, "bomb.png", "image/png", hugePNG(t))

		v.complete(id).AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "attachment_image_too_large")
	})

	t.Run("refuses a message carrying an upload still pending", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.upload(t, "notes.txt", "text/plain", []byte("hello"))

		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
			"client_id": ids.New().String(), "body": "see file", "attachment_ids": []string{id},
		}).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "attachment_not_uploaded")
	})

	t.Run("hides one visitor's upload from another", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v, other := startVisitor(t, tc), startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 5).AssertCreated().String("data.id")

		other.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("hello")).
			AssertNotFound().AssertJSONPath("code", "attachment_not_found")
		other.complete(id).AssertNotFound()
		other.complete("att_nonsense").AssertNotFound()
	})

	t.Run("only moves an upload to uploaded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 5).AssertCreated().String("data.id")

		v.client().PatchJSON("/widget/v1/attachments/"+id, testkit.JSON{"status": "pending"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_transition")
	})

	t.Run("takes no files when the widget does not", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 5).AssertCreated().String("data.id")
		patch(t, tc, testkit.JSON{"attachments_enabled": false})

		v.announce("b.txt", "text/plain", 5).AssertForbidden().AssertJSONPath("code", "attachments_disabled")
		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("hello")).AssertForbidden()
		v.complete(id).AssertForbidden()
	})

	t.Run("needs the visitor's session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().Send(fhttp.MethodPut, "/widget/v1/attachments/att_x/content", []byte("hello")).AssertUnauthorized()
	})
}

func TestWidgetAttachmentsOnObjectStorage(t *testing.T) {
	t.Parallel()

	endpoint := testkit.Env(t, "AWS_ENDPOINT_URL_S3")
	onBucket := func(t *testing.T, endpoint string) *tests.TestCase {
		t.Helper()

		return tests.New(t, tests.WithEnv(
			"DIMMI_FILESYSTEMS__DEFAULT=uploads",
			"DIMMI_FILESYSTEMS__DISKS__UPLOADS__DRIVER=s3",
			"DIMMI_FILESYSTEMS__DISKS__UPLOADS__BUCKET="+testkit.NewBucket(t),
			"DIMMI_FILESYSTEMS__DISKS__UPLOADS__FORCE_PATH_STYLE=true",
			"DIMMI_FILESYSTEMS__DISKS__UPLOADS__ENDPOINT="+endpoint,
		))
	}

	t.Run("signs an upload the browser sends straight to storage", func(t *testing.T) {
		t.Parallel()
		tc := onBucket(t, endpoint)
		v := startVisitor(t, tc)
		picture := pngOf(t, 2, 2)

		res := v.announce("shot.png", "image/png", len(picture)).AssertCreated()
		url := res.String("data.upload.url")
		require.True(t, strings.HasPrefix(url, endpoint), url)

		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)

		put, err := client.New().WithHeader("Content-Type", "image/png").Put(t.Context(), url, picture)
		require.NoError(t, err)
		require.Equal(t, fhttp.StatusOK, put.Status)

		v.complete(res.String("data.id")).AssertOK().AssertJSONPath("data.width", 2.0)
	})

	t.Run("asks the browser to retry while storage is down", func(t *testing.T) {
		t.Parallel()
		tc := onBucket(t, "http://127.0.0.1:1")
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 5).AssertCreated().String("data.id")

		res := v.complete(id).AssertStatus(fhttp.StatusServiceUnavailable).AssertJSONPath("code", "storage_unavailable")
		assert.Equal(t, "5", res.Header.Get("Retry-After"))

		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("hello")).
			AssertStatus(fhttp.StatusServiceUnavailable).AssertJSONPath("code", "storage_unavailable")
	})
}

func TestWidgetUploadEdges(t *testing.T) {
	t.Parallel()

	t.Run("bytes cannot be sent again once an upload is finished", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.upload(t, "a.txt", "text/plain", []byte("hi"))
		v.complete(id).AssertOK()

		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("hi")).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_transition")
	})

	t.Run("removes bytes that only pretend to be a picture", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		fake := slices.Concat(pngOf(t, 1, 1)[:16], bytes.Repeat([]byte{0xff}, 64))
		id := v.upload(t, "x.png", "image/png", fake)

		v.complete(id).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "attachment_type_blocked")
	})

	t.Run("a refused file whose row cannot be removed answers a server error", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 10).AssertCreated().String("data.id")
		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("short")).AssertNoContent()
		tc.FailWrites("attachments", "DELETE")

		v.complete(id).AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("sending bytes while uploads cannot be read answers a server error", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.announce("a.txt", "text/plain", 2).AssertCreated().String("data.id")
		tc.BreakTable("attachments")

		v.client().Send(fhttp.MethodPut, "/widget/v1/attachments/"+id+"/content", []byte("hi")).
			AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("finishing at once from several tabs answers each the same", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.upload(t, "a.txt", "text/plain", []byte("hi"))

		var wg sync.WaitGroup
		statuses := make([]int, 6)
		for i := range statuses {
			wg.Go(func() { statuses[i] = v.complete(id).Status })
		}
		wg.Wait()

		for _, status := range statuses {
			assert.Equal(t, fhttp.StatusOK, status)
		}
	})
}
