/*
|--------------------------------------------------------------------------
| Avatar Controller
|--------------------------------------------------------------------------
|
| Pictures of people, served from the application's own origin whichever
| disk holds them, so pages and mail clients never talk to a storage
| provider. A picture is a fresh random name forever, so it can be cached
| for good and cannot be guessed, and only names of that exact shape ever
| reach the disk.
|
*/

package web

import (
	"errors"
	"io"
	"regexp"

	"github.com/dimmihq/dimmi/framework/filesystem"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
avatarName is the shape of every stored picture's file name.
*/
var avatarName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.png$`)

/*
AvatarDisks finds the disk pictures are on.
*/
type AvatarDisks interface {
	/*
		Default returns the application's default disk.
	*/
	Default() (filesystem.Filesystem, error)
}

/*
AvatarController serves stored pictures.
*/
type AvatarController struct {
	Disks AvatarDisks
}

/*
Show writes the picture the path names, or answers not found.
*/
func (c *AvatarController) Show(w fhttp.ResponseWriter, r *fhttp.Request) {
	name := r.PathValue("file")
	if !avatarName.MatchString(name) {
		w.WriteHeader(fhttp.StatusNotFound)

		return
	}

	disk, err := c.Disks.Default()
	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)

		return
	}

	file, err := disk.Open(r.Context(), "avatars/"+name)
	if errors.Is(err, filesystem.ErrNotFound) {
		w.WriteHeader(fhttp.StatusNotFound)

		return
	}

	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)

		return
	}

	defer func() { _ = file.Close() }()

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, file)
}
