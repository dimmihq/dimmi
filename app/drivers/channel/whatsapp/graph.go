/*
|--------------------------------------------------------------------------
| Graph API
|--------------------------------------------------------------------------
|
| WhatsApp calls Meta through the Graph client every Meta channel shares.
| What is WhatsApp's own lives here: uploading a file to a number's media
| store, and asking where Meta serves a file a customer sent.
|
*/

package whatsapp

import (
	"context"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
)

/*
Errors of the Graph client. ErrMediaHost refuses a media address off
Meta's hosts, and ErrMediaUnavailable a file Meta no longer serves.
*/
var (
	ErrMediaHost        = meta.ErrMediaHost
	ErrMediaUnavailable = meta.ErrMediaUnavailable
)

/*
GraphError is a call Meta refused: its code and subcode, its own words,
and the HTTP status it came with.
*/
type GraphError = meta.GraphError

/*
Graph calls Meta's Graph API with one token.
*/
type Graph struct {
	*meta.Graph
}

/*
NewGraph returns the Graph API at the base URL and version, calling with
the token.
*/
func NewGraph(http *httpclient.Client, base, version, token string) *Graph {
	return &Graph{Graph: meta.NewGraph(http, base, version, token)}
}

/*
Upload sends one file to the number's media store and returns its id.
The file part's field is filled in here.
*/
func (g *Graph) Upload(ctx context.Context, phoneNumberID string, file httpclient.FilePart) (string, error) {
	file.Field = "file"
	fields := map[string]string{"messaging_product": "whatsapp", "type": file.ContentType}

	var out struct {
		ID string `json:"id"`
	}

	if err := g.Multipart(ctx, phoneNumberID+"/media", meta.Form{Fields: fields, File: file}, &out); err != nil {
		return "", err
	}

	return out.ID, nil
}

/*
Media is a file a customer sent, as Meta describes it.
*/
type Media struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"file_size"`
	SHA256   string `json:"sha256"`
}

/*
MediaInfo returns where Meta serves the media, for a few minutes.
*/
func (g *Graph) MediaInfo(ctx context.Context, mediaID string) (Media, error) {
	var out Media

	err := g.Get(ctx, mediaID, nil, &out)

	return out, err
}
