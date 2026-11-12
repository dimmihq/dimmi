/*
|--------------------------------------------------------------------------
| File Sources
|--------------------------------------------------------------------------
|
| Files on a storage service are read and uploaded through links to that
| service, not to the app. Pages may load pictures, play media and send
| uploads to the public addresses the disks name, and nowhere else.
|
*/

package providers

import (
	"net/url"
	"slices"

	"github.com/dimmihq/dimmi/app"
)

/*
fileSources returns the origins of the disks' public endpoints and file
URLs, each once.
*/
func fileSources(a *app.App) []string {
	var out []string

	for _, section := range a.Application.Config.Filesystems.Disks {
		for _, key := range []string{"public_endpoint", "url"} {
			raw := section.String(key)
			u, err := url.Parse(raw)
			if raw == "" || err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
				continue
			}

			if o := u.Scheme + "://" + u.Host; !slices.Contains(out, o) {
				out = append(out, o)
			}
		}
	}

	slices.Sort(out)

	return out
}
