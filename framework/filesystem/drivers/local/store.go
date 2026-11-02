/*
|--------------------------------------------------------------------------
| Local Store
|--------------------------------------------------------------------------
|
| Files are opened through an os.Root, so no path and no symbolic link
| can reach outside the disk's directory. A write lands in a temporary
| file and is renamed into place, so readers see the old file or the new
| one and never half of either, and a failed write leaves nothing.
|
| Each file's attributes sit beside it in a sidecar named after it with
| an .attrs suffix. That suffix is reserved: such paths are refused, and
| listings skip sidecars and the temporary files of writes in progress.
|
*/

package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/filesystem"
)

const (
	attrsSuffix = ".attrs"
	dirMode     = 0o750
	fileMode    = 0o600
)

/*
A store is the filesystem.Store of a local disk. Its signer is nil when
the disk cannot sign URLs.
*/
type store struct {
	root   *os.Root
	signer *signer
	clock  clock.Clock
}

/*
sidecar is the stored form of a file's attributes.
*/
type sidecar struct {
	ContentType     string            `json:"content_type"`
	CacheControl    string            `json:"cache_control,omitempty"`
	ContentEncoding string            `json:"content_encoding,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

var _ filesystem.Store = (*store)(nil)

/*
Write stores the file and its attributes, each through a temporary file
renamed into place. The file is renamed last, so a failed write leaves
the old file and its attributes as they were.
*/
func (s *store) Write(ctx context.Context, name string, r io.Reader, opts filesystem.PutOptions) error {
	if err := checkName(name); err != nil {
		return err
	}

	if dir := path.Dir(name); dir != "." {
		if err := s.root.MkdirAll(dir, dirMode); err != nil {
			return fmt.Errorf("create folder: %w", err)
		}
	}

	tmp, err := s.writeTemp(ctx, name, r)
	if err != nil {
		return err
	}

	err = s.writeAttrs(ctx, name, opts)
	if err == nil {
		err = s.root.Rename(tmp, name)
	}

	if err != nil {
		return errors.Join(err, ignoreMissing(s.root.Remove(tmp)))
	}

	return nil
}

/*
writeAttrs replaces the sidecar of the named file with the attributes
it is being written with.
*/
func (s *store) writeAttrs(ctx context.Context, name string, opts filesystem.PutOptions) error {
	attrs, err := json.Marshal(sidecar{
		ContentType:     opts.ContentType,
		CacheControl:    opts.CacheControl,
		ContentEncoding: opts.ContentEncoding,
		Metadata:        opts.Metadata,
	})
	if err != nil {
		return fmt.Errorf("encode attributes: %w", err)
	}

	tmp, err := s.writeTemp(ctx, name+attrsSuffix, bytes.NewReader(attrs))
	if err != nil {
		return err
	}

	if err := s.root.Rename(tmp, name+attrsSuffix); err != nil {
		return errors.Join(err, ignoreMissing(s.root.Remove(tmp)))
	}

	return nil
}

/*
writeTemp writes the reader to a new temporary file beside the name and
returns the temporary file's name. A failed write removes the file.
*/
func (s *store) writeTemp(ctx context.Context, name string, r io.Reader) (string, error) {
	tmp := name + "." + rand.Text() + ".tmp" + attrsSuffix

	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}

	_, err = io.Copy(f, r)
	if err = errors.Join(err, f.Close(), ctx.Err()); err != nil {
		return "", errors.Join(err, ignoreMissing(s.root.Remove(tmp)))
	}

	return tmp, nil
}

/*
Open opens the file for reading.
*/
func (s *store) Open(_ context.Context, name string) (io.ReadCloser, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}

	f, err := s.root.Open(name)
	if err != nil {
		return nil, mapError(err)
	}

	info, err := f.Stat()
	if err == nil && info.IsDir() {
		err = filesystem.ErrNotFound
	}

	if err != nil {
		return nil, errors.Join(err, f.Close())
	}

	return f, nil
}

/*
Stat reads the file's information and its sidecar. A file without a
sidecar, such as one copied in by hand, is reported as binary.
*/
func (s *store) Stat(_ context.Context, name string) (filesystem.Attributes, error) {
	info, err := s.regularFile(name)
	if err != nil {
		return filesystem.Attributes{}, err
	}

	attrs := sidecar{ContentType: "application/octet-stream"}

	raw, err := s.root.ReadFile(name + attrsSuffix)
	if err == nil {
		err = json.Unmarshal(raw, &attrs)
	}

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return filesystem.Attributes{}, fmt.Errorf("read attributes: %w", err)
	}

	return filesystem.Attributes{
		Size:            info.Size(),
		ContentType:     attrs.ContentType,
		CacheControl:    attrs.CacheControl,
		ContentEncoding: attrs.ContentEncoding,
		LastModified:    info.ModTime(),
		Metadata:        attrs.Metadata,
	}, nil
}

/*
Delete removes the file, then its sidecar.
*/
func (s *store) Delete(_ context.Context, name string) error {
	if _, err := s.regularFile(name); err != nil {
		return err
	}

	if err := s.root.Remove(name); err != nil {
		return mapError(err)
	}

	return ignoreMissing(s.root.Remove(name + attrsSuffix))
}

/*
List walks from the deepest folder the prefix names.
*/
func (s *store) List(ctx context.Context, prefix string) iter.Seq2[filesystem.Entry, error] {
	return func(yield func(filesystem.Entry, error) bool) {
		dir := "."
		if i := strings.LastIndex(prefix, "/"); i > 0 {
			dir = prefix[:i]
		}

		l := &listing{store: s, prefix: prefix, yield: yield}
		l.walk(ctx, dir)
	}
}

/*
Sign returns a URL under the signed URL base, or ErrUnsupported when the
disk has no signing key.
*/
func (s *store) Sign(_ context.Context, name string, req filesystem.SignRequest) (string, error) {
	if s.signer == nil {
		return "", filesystem.ErrUnsupported
	}

	if err := checkName(name); err != nil {
		return "", err
	}

	return s.signer.sign(name, req, s.clock.Now()), nil
}

/*
Close releases the disk's directory.
*/
func (s *store) Close() error {
	return s.root.Close()
}

/*
regularFile returns the information of the file at the name, and
ErrNotFound for a missing path or a folder.
*/
func (s *store) regularFile(name string) (fs.FileInfo, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}

	info, err := s.root.Stat(name)
	if err != nil {
		return nil, mapError(err)
	}

	if info.IsDir() {
		return nil, filesystem.ErrNotFound
	}

	return info, nil
}

/*
checkName refuses paths ending in the reserved sidecar suffix.
*/
func checkName(name string) error {
	if strings.HasSuffix(name, attrsSuffix) {
		return fmt.Errorf("%w: the %s suffix is reserved", filesystem.ErrInvalidPath, attrsSuffix)
	}

	return nil
}

/*
mapError reports a missing file, or a path running through a file as if
it were a folder, as ErrNotFound.
*/
func mapError(err error) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return fmt.Errorf("%w: %w", filesystem.ErrNotFound, err)
	}

	return err
}

func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}
