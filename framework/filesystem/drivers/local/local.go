/*
|--------------------------------------------------------------------------
| Local Disk
|--------------------------------------------------------------------------
|
| A local disk keeps files in a directory on this server. It is the right
| disk for a single server and for development and tests, and the wrong
| one as soon as there are two servers, because neither sees the other's
| files. Content type, cache headers and metadata live in a small file
| beside each stored file, so nothing is lost compared to S3.
|
| Temporary URLs need a signing key and a base URL under which the
| application serves signed links; without them the disk is private and
| refuses to sign.
|
*/

package local

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
ErrRootRequired is returned when a local disk names no directory.
*/
var ErrRootRequired = errors.New("local disk: root is required")

/*
Options configure a local disk.

Root is the directory holding the files; it is created when missing. URL
is the public base URL the files are served from, if any. SignedURLBase
and SigningKey together enable temporary URLs: links under the base
that the application's route checks with Verify.
*/
type Options struct {
	Root          string `koanf:"root"`
	URL           string `koanf:"url"`
	SignedURLBase string `koanf:"signed_url_base"`
	SigningKey    string `koanf:"signing_key"`
}

/*
Factory builds local disks for the disk manager.
*/
func Factory(_ string, section config.Section) (filesystem.Filesystem, error) {
	var opts Options
	if err := section.Decode(&opts); err != nil {
		return nil, err
	}

	return New(opts)
}

/*
New opens a local disk, creating its directory when missing.
*/
func New(opts Options) (*filesystem.Disk, error) {
	if opts.Root == "" {
		return nil, ErrRootRequired
	}

	s := &store{clock: clock.System()}

	if opts.SignedURLBase != "" && opts.SigningKey != "" {
		base, err := url.Parse(opts.SignedURLBase)
		if err != nil {
			return nil, fmt.Errorf("parse signed_url_base: %w", err)
		}

		s.signer = &signer{base: base, key: []byte(opts.SigningKey)}
	}

	if err := os.MkdirAll(opts.Root, dirMode); err != nil {
		return nil, fmt.Errorf("open local disk at %s: %w", opts.Root, err)
	}

	root, err := os.OpenRoot(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("open local disk at %s: %w", opts.Root, err)
	}

	s.root = root

	return filesystem.NewDisk(s, filesystem.DiskOptions{URL: opts.URL}), nil
}
