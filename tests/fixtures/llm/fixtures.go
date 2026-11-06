/*
|--------------------------------------------------------------------------
| LLM Fixtures
|--------------------------------------------------------------------------
|
| Provider answers as the providers send them, for the model drivers'
| tests: server-sent event streams with text, tool calls, usage and
| every way a stream can end, and the JSON bodies of finished replies
| and model lists. They hold no real key or conversation.
|
*/

package llm

import (
	"embed"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Files holds the fixtures.
*/
//go:embed *.sse *.json
var Files embed.FS

/*
Reply is a 200 response carrying the named fixture: an event stream
for a .sse file, JSON otherwise. An unknown name answers an empty
body, which fails the test that asked for it.
*/
func Reply(name string) httpclient.Response {
	data, _ := Files.ReadFile(name)
	contentType := "application/json"
	if strings.HasSuffix(name, ".sse") {
		contentType = "text/event-stream"
	}
	return httpclient.Response{
		Status: fhttp.StatusOK, Header: fhttp.Header{"Content-Type": {contentType}}, Body: data,
	}
}
