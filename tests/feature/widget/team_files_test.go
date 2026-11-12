package widget_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
teamUpload has the owner upload a file from the inbox and returns its id.
*/
func teamUpload(t *testing.T, tc *tests.TestCase, name, contentType string) func([]byte) *testkit.Response {
	t.Helper()

	return func(content []byte) *testkit.Response {
		return owner(tc).WithHeader("Content-Type", contentType).
			Send(fhttp.MethodPost, "/api/v1/attachments?filename="+name, content)
	}
}

/*
fetch reads a signed link the way a browser would, without any session.
*/
func fetch(t *testing.T, tc *tests.TestCase, link string) *testkit.Response {
	t.Helper()

	address, err := url.Parse(link)
	require.NoError(t, err)

	return tc.Client().Send(fhttp.MethodGet, address.RequestURI(), nil)
}

/*
linkOf replies with the uploaded file, lets the scan judge it, and returns
the link the timeline then gives the file.
*/
func linkOf(t *testing.T, tc *tests.TestCase, file string) string {
	t.Helper()

	conversation := startVisitor(t, tc).startConversation(t, "Can you send it?")
	owner(tc).PostJSON("/api/v1/conversations/"+conversation+"/replies", testkit.JSON{
		"body": "Here it is.", "attachment_ids": []string{file},
	}).AssertCreated()
	tc.Drain()

	timeline := owner(tc).GetJSON("/api/v1/conversations/" + conversation + "/messages").AssertOK()

	return timeline.String("data.#(body==\"Here it is.\").attachments.0.url")
}

func TestTeamFiles(t *testing.T) {
	t.Parallel()

	t.Run("a teammate uploads a picture and its link serves it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		picture := pngOf(t, 40, 30)

		res := teamUpload(t, tc, "shot.png", "image/png")(picture).AssertCreated()
		res.AssertJSONPath("data.filename", "shot.png").AssertJSONPath("data.width", float64(40))
		assert.Empty(t, res.String("data.url"))
		link := linkOf(t, tc, res.String("data.id"))
		require.NotEmpty(t, link)

		served := fetch(t, tc, link).AssertOK()
		assert.Equal(t, "image/png", served.Header.Get("Content-Type"))
		assert.Equal(t, picture, served.Body)
	})

	t.Run("a link that was tampered with finds nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		file := teamUpload(t, tc, "shot.png", "image/png")(pngOf(t, 4, 4)).AssertCreated().String("data.id")
		link := linkOf(t, tc, file)

		fetch(t, tc, strings.Replace(link, "signature=", "signature=00", 1)).AssertNotFound()
	})

	t.Run("a file that is not a picture always downloads", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		file := teamUpload(t, tc, "notes.txt", "text/plain")([]byte("hello")).AssertCreated().String("data.id")
		link := linkOf(t, tc, file)

		served := fetch(t, tc, link).AssertOK()
		assert.Equal(t, "attachment; filename=notes.txt", served.Header.Get("Content-Disposition"))
		assert.Equal(t, "nosniff", served.Header.Get("X-Content-Type-Options"))
	})

	t.Run("a reply carries the file to the visitor and into the timeline", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		conversation := v.startConversation(t, "Can you send the invoice?")
		file := teamUpload(t, tc, "invoice.pdf", "application/pdf")([]byte("%PDF-1.4 test")).AssertCreated().
			String("data.id")

		owner(tc).PostJSON("/api/v1/conversations/"+conversation+"/replies", testkit.JSON{
			"body": "Here it is.", "attachment_ids": []string{file},
		}).AssertCreated()
		tc.Drain()

		timeline := owner(tc).GetJSON("/api/v1/conversations/" + conversation + "/messages").AssertOK()
		timeline.AssertJSONPath("data.#(body==\"Here it is.\").attachments.0.filename", "invoice.pdf")

		seen := v.client().GetJSON("/widget/v1/conversations/" + conversation + "/messages").AssertOK()
		seen.AssertJSONPath("data.#(body==\"Here it is.\").attachments.0.filename", "invoice.pdf")
		assert.NotEmpty(t, seen.String("data.#(body==\"Here it is.\").attachments.0.url"))
	})

	t.Run("a reply may be a file alone, but not nothing at all", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		conversation := v.startConversation(t, "Screenshot please")
		file := teamUpload(t, tc, "shot.png", "image/png")(pngOf(t, 4, 4)).AssertCreated().String("data.id")
		path := "/api/v1/conversations/" + conversation + "/replies"

		owner(tc).PostJSON(path, testkit.JSON{"body": "", "attachment_ids": []string{file}}).AssertCreated()
		owner(tc).PostJSON(path, testkit.JSON{"body": " "}).AssertValidationError("body")
	})

	t.Run("a file goes with one reply only, and only the uploader's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		conversation := v.startConversation(t, "Hello")
		file := teamUpload(t, tc, "a.png", "image/png")(pngOf(t, 4, 4)).AssertCreated().String("data.id")
		path := "/api/v1/conversations/" + conversation + "/replies"

		owner(tc).PostJSON(path, testkit.JSON{"body": "one", "attachment_ids": []string{file}}).AssertCreated()
		owner(tc).PostJSON(path, testkit.JSON{"body": "two", "attachment_ids": []string{file}}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "attachment_not_uploaded")

		_, teammate := memberClient(t, tc, "member")
		mine := teammate.WithHeader("Content-Type", "image/png").
			Send(fhttp.MethodPost, "/api/v1/attachments?filename=b.png", pngOf(t, 4, 4)).AssertCreated().
			String("data.id")
		owner(tc).PostJSON(path, testkit.JSON{"body": "three", "attachment_ids": []string{mine}}).
			AssertStatus(fhttp.StatusConflict)
	})

	t.Run("a file that is not what it claims is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		teamUpload(t, tc, "run.exe", "application/octet-stream")([]byte("MZ\x90\x00")).
			AssertJSONPath("code", "attachment_type_blocked")
	})
}
