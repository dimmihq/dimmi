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
| The browser app reports its own errors to a Sentry-compatible tracker
| when ErrorsDSN names one. The page hands it the DSN with the server's
| environment and release, so reports from a build read as that build.
|
| The browser app records its signed-in teammates' sessions on a kimmi
| server when SessionsHost and SessionsKey name one. The page hands it
| both, and the app loads the recorder from that host. Neither is set by
| default, so a server records nothing until its operator names one.
| With SessionsProjectID and SessionsToken also set, the server erases a
| workspace's recordings on the recorder when it purges the workspace.
|
*/

package config

import (
	"fmt"
	"net/url"
)

/*
FrontendConfig configures the browser app. HotPath is the file the
Vite dev server writes its address to, empty outside development.
The key avoids a _FILE ending, which the environment reads as a file
holding the value. ErrorsDSN is the DSN the browser app reports errors
to, empty to report nothing. SessionsHost is the origin of the kimmi
server the browser app records sessions on, such as
https://sessions.example.com, and SessionsKey the public key of its
project; both empty to record nothing. SessionsProjectID is the id of
that project and SessionsToken a write API token of the recorder, which
the server uses to erase a purged workspace's recordings; either empty
to erase nothing there.
*/
type FrontendConfig struct {
	HotPath           string `koanf:"hot_path"`
	ErrorsDSN         string `koanf:"errors_dsn"`
	SessionsHost      string `koanf:"sessions_host"`
	SessionsKey       string `koanf:"sessions_key"`
	SessionsProjectID string `koanf:"sessions_project_id"`
	SessionsToken     string `koanf:"sessions_token"      secret:"true"`
}

/*
Records reports whether the browser app records sessions.
*/
func (c FrontendConfig) Records() bool {
	return c.SessionsHost != "" && c.SessionsKey != ""
}

/*
check reports a session recorder that cannot work: a host without a key
or a key without a host, or a host that is not a bare http or https
origin.
*/
func (c FrontendConfig) check() error {
	if c.SessionsHost == "" && c.SessionsKey == "" {
		return nil
	}

	u, err := url.Parse(c.SessionsHost)
	origin := err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" &&
		u.User == nil && (u.Path == "" || u.Path == "/") && u.RawQuery == "" && u.Fragment == ""

	if !origin || c.SessionsKey == "" {
		return fmt.Errorf("%w: frontend.sessions_host", ErrInvalid)
	}

	return nil
}

/*
SessionsOrigin returns the recorder's origin, without a trailing slash,
or empty when the browser app records nothing.
*/
func (c FrontendConfig) SessionsOrigin() string {
	if !c.Records() {
		return ""
	}

	u, err := url.Parse(c.SessionsHost)
	if err != nil {
		return ""
	}

	return u.Scheme + "://" + u.Host
}

/*
SessionsEraseURL returns the recorder's address that erases visitors of
its project, or empty when the server erases nothing there: no recorder,
no project id or no token.
*/
func (c FrontendConfig) SessionsEraseURL() string {
	origin := c.SessionsOrigin()
	if origin == "" || c.SessionsProjectID == "" || c.SessionsToken == "" {
		return ""
	}

	return origin + "/api/v1/projects/" + url.PathEscape(c.SessionsProjectID) + "/visitors/erase"
}
