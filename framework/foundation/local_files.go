/*
|--------------------------------------------------------------------------
| Local File Links
|--------------------------------------------------------------------------
|
| A file on a local disk has no storage service to hand out links, so the
| application signs them itself and serves them back. Every local disk
| gets a signing key derived from the app key and a link base under this
| server's own address, unless its configuration names its own. A link
| reads, or for an upload writes, exactly one file until it expires;
| anything unsigned, altered or late is refused. Files that are not
| pictures are always downloaded, never shown inline, so nothing uploaded
| can run as a page of this site.
|
*/

package foundation

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
)

/*
localFilesPath is where signed links to local files are served.
*/
const localFilesPath = "/files/"

/*
maxLocalUpload caps one upload through a signed link; the caller that
asked for the link checks the file's real size afterwards.
*/
const maxLocalUpload = 512 << 20

/*
inlineTypes are the file types a browser may show in place; everything
else downloads.
*/
func inlineTypes() []string {
	return []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/avif"}
}

/*
signLocalDisks gives every local disk without its own link settings a
signing key derived from the app key and a link base under the app's
address.
*/
func (a *Application) signLocalDisks() error {
	for name, section := range a.Config.Filesystems.Disks {
		if section.String("driver") != "local" || section.String("signing_key") != "" {
			continue
		}

		key, err := encryption.DeriveKey(a.appKey, "local-disk-links:"+name)
		if err != nil {
			return fmt.Errorf("derive link key for disk %s: %w", name, err)
		}

		signed := maps.Clone(section)

		signed["signing_key"] = fmt.Sprintf("%x", key)
		if section.String("signed_url_base") == "" {
			signed["signed_url_base"] = strings.TrimSuffix(a.Config.App.URL, "/") + localFilesPath + name
		}

		a.Config.Filesystems.Disks[name] = signed
	}

	return nil
}

/*
serveLocalFiles registers the route that answers signed links to local
disks.
*/
func (a *Application) serveLocalFiles() {
	a.Router.Handle(http.MethodGet, localFilesPath+"{disk}/*", "files.show", http.HandlerFunc(a.localFile))
	a.Router.Handle(http.MethodHead, localFilesPath+"{disk}/*", "files.head", http.HandlerFunc(a.localFile))
	a.Router.Handle(http.MethodPut, localFilesPath+"{disk}/*", "files.update", http.HandlerFunc(a.localFile))
	a.Router.Handle(http.MethodOptions, localFilesPath+"{disk}/*", "files.preflight", http.HandlerFunc(preflight))
}

/*
preflight lets any site send a signed upload; the signature, not the
site, decides whether it lands.
*/
func preflight(w http.ResponseWriter, _ *http.Request) {
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Methods", "GET, HEAD, PUT")
	header.Set("Access-Control-Allow-Headers", "Content-Type")
	header.Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
}

/*
localFile streams the file a signed link names, or answers not found for
any link that does not check out, so a probe learns nothing.
*/
func (a *Application) localFile(w http.ResponseWriter, r *http.Request) {
	name, path := r.PathValue("disk"), r.PathValue("*")
	section, ok := a.Config.Filesystems.Disks[name]
	if !ok || section.String("driver") != "local" || section.String("signing_key") == "" {
		http.NotFound(w, r)

		return
	}

	if err := local.Verify(r, path, section.String("signing_key"), clock.System().Now()); err != nil {
		http.NotFound(w, r)

		return
	}

	disk, err := a.Storage.Disk(name)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	if r.Method == http.MethodPut {
		storeLocalFile(w, r, disk, path)

		return
	}

	a.streamLocalFile(w, r, disk, path)
}

/*
storeLocalFile writes the body of a signed upload to the path, with the
type the link was signed for.
*/
func storeLocalFile(w http.ResponseWriter, r *http.Request, disk filesystem.Filesystem, path string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	body := http.MaxBytesReader(w, r.Body, maxLocalUpload)
	err := disk.PutStream(r.Context(), path, body, filesystem.PutOptions{ContentType: r.Header.Get("Content-Type")})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusOK)
}

/*
streamLocalFile writes the file with headers that keep it a file: its
own type, never sniffed, downloaded unless it is a picture, and readable
from any site since the signature already decides who may read it.
*/
func (a *Application) streamLocalFile(w http.ResponseWriter, r *http.Request, disk filesystem.Filesystem, path string) {
	attrs, err := disk.Stat(r.Context(), path)
	if errors.Is(err, filesystem.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	header := w.Header()
	header.Set("Content-Type", attrs.ContentType)
	header.Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	header.Set("Cross-Origin-Resource-Policy", "cross-origin")
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Cache-Control", "private, max-age=300")

	if !inline(attrs.ContentType) {
		header.Set("Content-Disposition", "attachment")
	}

	if r.Method == http.MethodHead {
		return
	}

	file, err := disk.Open(r.Context(), path)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	defer func() { _ = file.Close() }()

	_, _ = io.Copy(w, file)
}

/*
inline reports whether a browser may show the type in place.
*/
func inline(contentType string) bool {
	base, _, _ := strings.Cut(contentType, ";")

	for _, t := range inlineTypes() {
		if strings.EqualFold(strings.TrimSpace(base), t) {
			return true
		}
	}

	return false
}
