/*
|--------------------------------------------------------------------------
| Multipart Uploads
|--------------------------------------------------------------------------
|
| Some APIs take a file as a multipart form: plain fields beside one file
| part with its own name and type. The form is built here, in memory, so
| callers outside the framework never assemble multipart bodies or MIME
| headers themselves.
|
*/

package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/textproto"
	"slices"
)

/*
FilePart is the file of a multipart form: the form field it fills, its
name, its content type and its content.
*/
type FilePart struct {
	Field       string
	Filename    string
	ContentType string
	Content     io.Reader
}

/*
PostMultipart sends the fields and the file as a multipart form. Fields
are written in key order, before the file.
*/
func (r *Request) PostMultipart(ctx context.Context, rawURL string, fields map[string]string, file FilePart) (
	*Response, error,
) {
	var buf bytes.Buffer

	form := multipart.NewWriter(&buf)

	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if err := form.WriteField(key, fields[key]); err != nil {
			return nil, fmt.Errorf("build multipart form: %w", err)
		}
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, file.Field, file.Filename))
	header.Set("Content-Type", file.ContentType)

	part, err := form.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("build multipart form: %w", err)
	}

	if _, err := io.Copy(part, file.Content); err != nil {
		return nil, fmt.Errorf("read multipart file: %w", err)
	}

	if err := form.Close(); err != nil {
		return nil, fmt.Errorf("build multipart form: %w", err)
	}

	return r.WithHeader("Content-Type", form.FormDataContentType()).Post(ctx, rawURL, buf.Bytes())
}
