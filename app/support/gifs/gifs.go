/*
|--------------------------------------------------------------------------
| GIFs In Replies
|--------------------------------------------------------------------------
|
| A teammate may send a GIF found through GIPHY's search. Such a picture
| is a link to GIPHY's media servers, kept exactly as GIPHY returned it.
| Any other remote picture in a reply could track the customer or show
| something nobody checked, so it is reduced to its words, and so are
| GIPHY's pictures when the instance has GIF search off.
|
*/

package gifs

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/markdown"
)

/*
Provider names where the GIFs come from.
*/
const Provider = "giphy"

/*
Hosted reports whether the address is a picture on GIPHY's servers,
over HTTPS.
*/
func Hosted(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" {
		return false
	}

	host := strings.ToLower(u.Hostname())

	return host == "giphy.com" || strings.HasSuffix(host, ".giphy.com")
}

/*
Never keeps no remote picture, for when GIF search is off.
*/
func Never(string) bool {
	return false
}

/*
Images returns the rewrite a reply's pictures pass through on their way
out: a picture of this server, such as an uploaded image, stays; a
remote one stays only when keep says so, such as Hosted while GIF search
is on; any other keeps only its words.
*/
func Images(keep func(address string) bool) func(markdown.Link) markdown.Link {
	return func(link markdown.Link) markdown.Link {
		if !link.Image {
			return link
		}

		u, err := url.Parse(link.URL)
		if err == nil && u.Host == "" && u.Scheme == "" {
			return link
		}

		if keep(link.URL) {
			return link
		}

		link.URL = ""

		return link
	}
}

/*
Metadata returns what a message records about the GIF its body sends,
{"gif": {"provider": "giphy", "id": "..."}}, or nil when it sends none.
*/
func Metadata(body string) json.RawMessage {
	id := find(markdown.Parse([]byte(body)))
	if id == "" {
		return nil
	}

	raw, err := json.Marshal(map[string]any{"gif": map[string]string{"provider": Provider, "id": id}})
	if err != nil {
		return nil
	}

	return raw
}

/*
find returns the GIPHY id of the first GIF under the node, empty when
there is none.
*/
func find(n *markdown.Node) string {
	if n.Kind == markdown.KindImage && Hosted(n.URL) {
		return idOf(n.URL)
	}

	for _, child := range n.Children {
		if id := find(child); id != "" {
			return id
		}
	}

	return ""
}

/*
idOf reads the GIF's id from a GIPHY media address, the folder the file
sits in, as in https://media1.giphy.com/media/{id}/giphy.gif.
*/
func idOf(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}

	return parts[len(parts)-2]
}
