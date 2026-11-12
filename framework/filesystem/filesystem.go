/*
|--------------------------------------------------------------------------
| Filesystem
|--------------------------------------------------------------------------
|
| Files live on named disks: attachments, exports, public assets. A disk
| is a local directory on a single server or an S3-compatible bucket once
| there are several, and application code never learns which one it has.
| Moving from one to the other is a configuration change.
|
| Paths are slash-separated and relative. A path built from user input
| can never climb out of its disk: empty, absolute and dotted paths are
| refused before any storage call is made.
|
*/

package filesystem

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"iter"
	"time"
)

/*
Errors a disk returns.

ErrNotFound means no file exists at the path. ErrInvalidPath refuses a
path that could escape the disk. ErrNoPublicURL comes from URL on a disk
whose files are private. ErrUnsupported means the disk cannot do what was
asked, such as signing URLs on a local disk without a signing key.
*/
var (
	ErrNotFound    = errors.New("filesystem: file not found")
	ErrInvalidPath = errors.New("filesystem: invalid path")
	ErrNoPublicURL = errors.New("filesystem: disk has no public url")
	ErrUnsupported = errors.New("filesystem: not supported by this disk")
)

/*
Filesystem is one disk.

Writes replace whatever was at the path. A failed streamed write leaves
nothing behind. Deleting a missing file is not an error. List yields files
in path order.
*/
type Filesystem interface {
	/*
		Put writes the contents to the path.
	*/
	Put(ctx context.Context, path string, contents []byte, opts PutOptions) error

	/*
		PutStream writes everything read from the reader to the path.
	*/
	PutStream(ctx context.Context, path string, r io.Reader, opts PutOptions) error

	/*
		Get reads the whole file at the path.
	*/
	Get(ctx context.Context, path string) ([]byte, error)

	/*
		Open returns a reader for the file at the path; the caller closes it.
	*/
	Open(ctx context.Context, path string) (io.ReadCloser, error)

	/*
		Exists reports whether a file exists at the path.
	*/
	Exists(ctx context.Context, path string) (bool, error)

	/*
		Stat returns the attributes of the file at the path.
	*/
	Stat(ctx context.Context, path string) (Attributes, error)

	/*
		Delete removes the file at the path.
	*/
	Delete(ctx context.Context, path string) error

	/*
		List yields every file whose path starts with the prefix.
	*/
	List(ctx context.Context, prefix string) iter.Seq2[Entry, error]

	/*
		URL returns the permanent public URL of the path.
	*/
	URL(path string) (string, error)

	/*
		TemporaryURL returns a signed URL that reads the path until the TTL
		passes.
	*/
	TemporaryURL(ctx context.Context, path string, ttl time.Duration) (string, error)

	/*
		DownloadURL returns a signed URL that reads the path until the TTL
		passes, under the file's own name: shown in place when Inline,
		saved otherwise.
	*/
	DownloadURL(ctx context.Context, path string, opts DownloadURLOptions) (string, error)

	/*
		TemporaryUploadURL returns a signed URL that accepts one PUT of the
		path.
	*/
	TemporaryUploadURL(ctx context.Context, path string, opts UploadURLOptions) (string, error)
}

/*
PutOptions describe a file being written.

The zero value is valid: the content type is then detected from the
first bytes written. Metadata keys are stored lowercase, because S3
lowercases them.
*/
type PutOptions struct {
	ContentType     string
	CacheControl    string
	ContentEncoding string
	Metadata        map[string]string
}

/*
DownloadURLOptions configure a signed download URL: how long it lasts,
the name the file is saved or shown under, and whether a browser may
show it in place.
*/
type DownloadURLOptions struct {
	TTL      time.Duration
	Filename string
	Inline   bool
}

/*
UploadURLOptions configure a signed upload URL. When ContentType is set,
the upload must send exactly that Content-Type.
*/
type UploadURLOptions struct {
	TTL         time.Duration
	ContentType string
}

/*
Attributes describe a stored file.
*/
type Attributes struct {
	Size            int64
	ContentType     string
	CacheControl    string
	ContentEncoding string
	LastModified    time.Time
	Metadata        map[string]string
}

/*
Entry is one file yielded by List.
*/
type Entry struct {
	Path         string
	Size         int64
	LastModified time.Time
}

/*
ValidatePath returns ErrInvalidPath unless the path names a file inside
the disk: relative, slash-separated, with no empty, "." or ".." parts.
*/
func ValidatePath(path string) error {
	if path == "." || !fs.ValidPath(path) {
		return ErrInvalidPath
	}

	return nil
}
