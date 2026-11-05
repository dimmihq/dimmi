/*
|--------------------------------------------------------------------------
| Disk
|--------------------------------------------------------------------------
|
| Every disk is the same Disk over a different store. The Disk refuses
| bad paths, detects content types, maps errors and builds public URLs,
| so a store only moves bytes and every disk behaves the same: missing
| files, signed URLs and errors all look identical to the code calling
| them, whether the bytes sit in a local directory or in S3.
|
*/

package filesystem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/*
DefaultSignedURLTTL is how long a signed URL lasts when no TTL is given.
*/
const DefaultSignedURLTTL = time.Hour

/*
sniffLen is how many leading bytes content type detection looks at.
*/
const sniffLen = 512

/*
Store is the storage under a Disk. Paths reaching a store are already
valid and every write carries its content type. A missing file is
reported as ErrNotFound and anything the store cannot do as
ErrUnsupported.
*/
type Store interface {
	/*
		Write stores everything read from the reader at the path, leaving
		nothing behind when the reader or the store fails.
	*/
	Write(ctx context.Context, path string, r io.Reader, opts PutOptions) error

	/*
		Open returns a reader for the file at the path.
	*/
	Open(ctx context.Context, path string) (io.ReadCloser, error)

	/*
		Stat returns the attributes of the file at the path.
	*/
	Stat(ctx context.Context, path string) (Attributes, error)

	/*
		Delete removes the file at the path; a missing file is not an error.
	*/
	Delete(ctx context.Context, path string) error

	/*
		List yields every file whose path starts with the prefix, in path
		order.
	*/
	List(ctx context.Context, prefix string) iter.Seq2[Entry, error]

	/*
		Sign returns a URL that performs one request on the path.
	*/
	Sign(ctx context.Context, path string, req SignRequest) (string, error)
}

/*
SignRequest describes a signed URL. Method is GET to read or PUT to
upload. ContentType, when set on an upload, is the only Content-Type the
upload may send.
*/
type SignRequest struct {
	Method      string
	TTL         time.Duration
	ContentType string
}

/*
DiskOptions configure a disk. URL is the public base URL of the files,
such as a CDN origin; empty means the files are private.
*/
type DiskOptions struct {
	URL string
}

/*
Disk is a Filesystem over a store.
*/
type Disk struct {
	store   Store
	baseURL string
}

var _ Filesystem = (*Disk)(nil)

/*
NewDisk returns a disk keeping its files in the store.
*/
func NewDisk(store Store, opts DiskOptions) *Disk {
	return &Disk{store: store, baseURL: strings.TrimSuffix(opts.URL, "/")}
}

/*
Put writes the contents to the path, replacing any file there.
*/
func (d *Disk) Put(ctx context.Context, path string, contents []byte, opts PutOptions) error {
	if err := ValidatePath(path); err != nil {
		return err
	}

	if opts.ContentType == "" {
		opts.ContentType = http.DetectContentType(contents)
	}

	return wrap("put", path, d.store.Write(ctx, path, bytes.NewReader(contents), normalize(opts)))
}

/*
PutStream writes everything read from the reader to the path. When the
read fails no file is left behind.
*/
func (d *Disk) PutStream(ctx context.Context, path string, r io.Reader, opts PutOptions) error {
	if err := ValidatePath(path); err != nil {
		return err
	}

	r, opts, err := sniff(r, opts)
	if err != nil {
		return wrap("put", path, err)
	}

	return wrap("put", path, d.store.Write(ctx, path, r, normalize(opts)))
}

/*
Get reads the whole file at the path.
*/
func (d *Disk) Get(ctx context.Context, path string) ([]byte, error) {
	r, err := d.Open(ctx, path)
	if err != nil {
		return nil, err
	}

	contents, err := io.ReadAll(r)
	closeErr := r.Close()

	return contents, wrap("get", path, errors.Join(err, closeErr))
}

/*
Open returns a reader for the file at the path.
*/
func (d *Disk) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := ValidatePath(path); err != nil {
		return nil, err
	}

	r, err := d.store.Open(ctx, path)
	if err != nil {
		return nil, wrap("open", path, err)
	}

	return r, nil
}

/*
Exists reports whether a file exists at the path.
*/
func (d *Disk) Exists(ctx context.Context, path string) (bool, error) {
	if err := ValidatePath(path); err != nil {
		return false, err
	}

	_, err := d.store.Stat(ctx, path)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}

	return err == nil, wrap("check", path, err)
}

/*
Stat returns the attributes of the file at the path.
*/
func (d *Disk) Stat(ctx context.Context, path string) (Attributes, error) {
	if err := ValidatePath(path); err != nil {
		return Attributes{}, err
	}

	attrs, err := d.store.Stat(ctx, path)
	if err != nil {
		return Attributes{}, wrap("stat", path, err)
	}

	if len(attrs.Metadata) == 0 {
		attrs.Metadata = nil
	}

	return attrs, nil
}

/*
Delete removes the file at the path. A missing file is already deleted.
*/
func (d *Disk) Delete(ctx context.Context, path string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}

	err := d.store.Delete(ctx, path)
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	return wrap("delete", path, err)
}

/*
List yields every file whose path starts with the prefix, in path order.
A prefix may end with a slash to list one folder.
*/
func (d *Disk) List(ctx context.Context, prefix string) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		if prefix != "" {
			if err := ValidatePath(strings.TrimSuffix(prefix, "/")); err != nil {
				yield(Entry{}, err)

				return
			}
		}

		for entry, err := range d.store.List(ctx, prefix) {
			if !yield(entry, wrap("list", prefix, err)) || err != nil {
				return
			}
		}
	}
}

/*
URL returns the permanent public URL of the path, or ErrNoPublicURL on a
private disk.
*/
func (d *Disk) URL(path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}

	if d.baseURL == "" {
		return "", ErrNoPublicURL
	}

	return d.baseURL + "/" + EscapePath(path), nil
}

/*
TemporaryURL returns a signed URL that reads the path until the TTL
passes, or DefaultSignedURLTTL when the TTL is not positive.
*/
func (d *Disk) TemporaryURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	return d.sign(ctx, path, SignRequest{Method: http.MethodGet, TTL: ttl})
}

/*
TemporaryUploadURL returns a signed URL that accepts one PUT of the path.
*/
func (d *Disk) TemporaryUploadURL(ctx context.Context, path string, opts UploadURLOptions) (string, error) {
	return d.sign(ctx, path, SignRequest{Method: http.MethodPut, TTL: opts.TTL, ContentType: opts.ContentType})
}

/*
Close releases the store's resources when it holds any.
*/
func (d *Disk) Close() error {
	if closer, ok := d.store.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

func (d *Disk) sign(ctx context.Context, path string, req SignRequest) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}

	if req.TTL <= 0 {
		req.TTL = DefaultSignedURLTTL
	}

	signed, err := d.store.Sign(ctx, path, req)

	return signed, wrap("sign", path, err)
}

/*
EscapePath escapes each segment of a path for use in a URL, keeping the
slashes between them.
*/
func EscapePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return strings.Join(segments, "/")
}

func wrap(op, path string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s %q: %w", op, path, err)
}

/*
sniff detects the content type from the first bytes of the reader when
none is given, and returns a reader that still yields every byte.
*/
func sniff(r io.Reader, opts PutOptions) (io.Reader, PutOptions, error) {
	if opts.ContentType != "" {
		return r, opts, nil
	}

	head := make([]byte, sniffLen)

	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, opts, err
	}

	opts.ContentType = DetectContentType(head[:n])

	return io.MultiReader(bytes.NewReader(head[:n]), r), opts, nil
}

/*
normalize lowercases metadata keys, as S3 stores them, so every disk
returns the same keys.
*/
func normalize(opts PutOptions) PutOptions {
	if len(opts.Metadata) == 0 {
		opts.Metadata = nil

		return opts
	}

	metadata := make(map[string]string, len(opts.Metadata))
	for key, value := range opts.Metadata {
		metadata[strings.ToLower(key)] = value
	}

	opts.Metadata = metadata

	return opts
}

/*
DetectContentType names the type of content from its first 512 bytes, the
way browsers sniff it, falling back to application/octet-stream.
*/
func DetectContentType(head []byte) string {
	return http.DetectContentType(head)
}
