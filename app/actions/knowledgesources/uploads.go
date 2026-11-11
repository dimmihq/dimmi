/*
|--------------------------------------------------------------------------
| Uploaded Archives
|--------------------------------------------------------------------------
|
| A Markdown source reads a file uploaded first: a zip of Markdown files
| or one Markdown file, kept in the workspace's own folder of the
| storage. A source may only read an upload of its own workspace, so one
| workspace can never point a source at another's archive.
|
*/

package knowledgesources

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/upload"
)

/*
maxUpload is the largest archive accepted.
*/
const maxUpload = 50 << 20

/*
ErrUploadInvalid refuses an upload that is not a zip or Markdown file,
or is too large.
*/
var ErrUploadInvalid = fault.New(fault.Invalid, "knowledge_upload_invalid",
	"Upload a .zip of Markdown files, or one .md file, of at most 50 MB.").OnField("file")

/*
StoreUpload keeps an uploaded archive for a source to read and returns
the key its settings name it by.
*/
func (e *Engine) StoreUpload(ctx context.Context, m authz.Membership, name string, r io.Reader) (string, error) {
	if !m.Can("knowledge:manage") {
		return "", exceptions.ErrForbidden
	}

	if e.Uploads == nil {
		return "", ErrUploadInvalid
	}

	ext := ""

	for _, allowed := range []string{".zip", ".md", ".mdx", ".markdown"} {
		if strings.HasSuffix(strings.ToLower(name), allowed) {
			ext = allowed
		}
	}

	if ext == "" {
		return "", ErrUploadInvalid
	}

	body, err := io.ReadAll(io.LimitReader(r, maxUpload+1))
	if err != nil || len(body) > maxUpload || len(body) == 0 {
		return "", ErrUploadInvalid
	}

	key := uploadPrefix(m.Workspace.ID) + ids.NewV7At(e.Clock.Now()).String() + ext
	if err := e.Uploads.Put(ctx, key, body, filesystem.PutOptions{}); err != nil {
		return "", fmt.Errorf("store upload: %w", err)
	}

	return key, nil
}

/*
uploadPrefix is the workspace's folder of uploads.
*/
func uploadPrefix(ws ids.UUID) string {
	return upload.Prefix + ws.String() + "/"
}

/*
checkUpload refuses settings naming an upload of another workspace.
*/
func checkUpload(ws ids.UUID, plain map[string]any) error {
	key, ok := plain["upload"].(string)
	if !ok || strings.HasPrefix(key, uploadPrefix(ws)) {
		return nil
	}

	return ErrUploadInvalid
}
