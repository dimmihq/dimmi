/*
|--------------------------------------------------------------------------
| llms.txt
|--------------------------------------------------------------------------
|
| /llms.txt tells coding assistants how to add the messenger to an app
| and call the API. It is the SDKs' own llms.txt, built into the binary,
| so a self-hosted install serves the copy matching its version. It
| changes only with a deploy, so browsers and crawlers keep it an hour.
|
*/

package web

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/resources/llms"
)

/*
LLMs serves llms.txt.
*/
func LLMs(w fhttp.ResponseWriter, _ *fhttp.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(llms.Text)
}
