/*
|--------------------------------------------------------------------------
| Frontend Settings
|--------------------------------------------------------------------------
|
| Where the browser app's scripts come from. A release serves the build
| embedded in the binary. During development the Vite dev server writes
| its address to a hot file, and while that file exists pages load the
| app from the dev server instead, with hot updates. Only a development
| setup names the file, so a production server never looks for one.
|
*/

package config

/*
FrontendConfig configures the browser app. HotPath is the file the
Vite dev server writes its address to, empty outside development.
The key avoids a _FILE ending, which the environment reads as a file
holding the value.
*/
type FrontendConfig struct {
	HotPath string `koanf:"hot_path"`
}
