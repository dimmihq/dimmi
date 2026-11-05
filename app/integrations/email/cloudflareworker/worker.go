/*
|--------------------------------------------------------------------------
| Cloudflare Worker
|--------------------------------------------------------------------------
|
| Cloudflare Email Routing hands mail to a Worker and has no webhook of its
| own, so the operator deploys this one. The script stays in the binary and
| is printed, never uploaded: the operator's Cloudflare account is theirs.
| The secret is never written into either file.
|
*/

package cloudflareworker

import (
	_ "embed"
	"strings"
)

//go:embed worker.js
var script string

//go:embed wrangler.toml
var wrangler string

/*
webhookPath is where the install takes the Worker's posts.
*/
const webhookPath = "/webhooks/cloudflare/inbound"

/*
Files are the two files to deploy.
*/
type Files struct {
	Script string
	Config string
}

/*
Render returns the files with the install's address filled in.
*/
func Render(appURL string) Files {
	address := strings.TrimRight(appURL, "/") + webhookPath

	return Files{Script: script, Config: strings.ReplaceAll(wrangler, "{{webhook_url}}", address)}
}
