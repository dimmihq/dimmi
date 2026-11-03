/*
|--------------------------------------------------------------------------
| Widget Settings
|--------------------------------------------------------------------------
|
| How the messenger widget's files reach customers' pages. The binary
| serves them itself; an install behind a CDN names the CDN's origin, and
| the loader then fetches the widget's hashed files from there while the
| loader itself keeps its fixed address on the install.
|
*/

package config

import "time"

/*
WidgetConfig configures the messenger widget. AssetsBaseURL is the
origin serving the widget's hashed files, such as a CDN in front of
this install; empty means the install's own host. LoaderMaxAge is how
long browsers and proxies keep the loader before asking again, which
bounds how long a deploy takes to reach every page.
*/
type WidgetConfig struct {
	AssetsBaseURL string        `koanf:"assets_base_url"`
	LoaderMaxAge  time.Duration `default:"5m"            koanf:"loader_max_age"`
}
