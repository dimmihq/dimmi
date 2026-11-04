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
	"net/http"
	"regexp"

	"github.com/dimmihq/dimmi/framework/filesystem"
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
func (c *AvatarController) Show(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !avatarName.MatchString(name) {
		http.NotFound(w, r)

		return
	}

	disk, err := c.Disks.Default()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	file, err := disk.Open(r.Context(), "avatars/"+name)
	if errors.Is(err, filesystem.ErrNotFound) {
		http.NotFound(w, r)

		return
	}

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	defer func() { _ = file.Close() }()

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, file)
}
