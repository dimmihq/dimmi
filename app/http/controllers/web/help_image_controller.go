/*
|--------------------------------------------------------------------------
| Help Image Controller
|--------------------------------------------------------------------------
|
| Pictures placed in articles, served from the application's own origin
| whichever disk holds them, on every host a help center answers on. A
| picture is a fresh random name forever, so it can be cached for good
| and cannot be guessed, and only names of that exact shape reach the
| disk.
|
*/

package web

import (
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/filesystem"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
helpImageName is the shape of every stored article picture's name.
*/
var helpImageName = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(png|jpg|gif)$`)

/*
HelpImageController serves article pictures.
*/
type HelpImageController struct {
	Disks AvatarDisks
	Dir   string
}

/*
Show writes the picture the path names, or answers not found.
*/
func (c *HelpImageController) Show(w fhttp.ResponseWriter, r *fhttp.Request) {
	name := r.PathValue("file")
	if !helpImageName.MatchString(name) {
		w.WriteHeader(fhttp.StatusNotFound)
		return
	}

	disk, err := c.Disks.Default()
	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)
		return
	}

	file, err := disk.Open(r.Context(), c.Dir+"/"+name)
	if errors.Is(err, filesystem.ErrNotFound) {
		w.WriteHeader(fhttp.StatusNotFound)
		return
	}

	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)
		return
	}

	defer func() { _ = file.Close() }()

	w.Header().Set("Content-Type", imageType(name))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, file)
}

/*
imageType is the media type a picture's name stands for.
*/
func imageType(name string) string {
	types := map[string]string{"png": "image/png", "jpg": "image/jpeg", "gif": "image/gif"}

	return types[name[strings.LastIndexByte(name, '.')+1:]]
}
