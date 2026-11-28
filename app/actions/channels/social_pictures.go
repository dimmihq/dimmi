/*
|--------------------------------------------------------------------------
| Pages' And Accounts' Pictures
|--------------------------------------------------------------------------
|
| Meta names a Page's or an Instagram account's picture by a signed
| address on its own hosts that stops working within days, and that a
| browser blocking Meta's hosts never loads. So the browser is never
| sent there: the server reads the picture itself. The accounts a
| sign-in shared carry theirs inline, small, since most are never
| connected; a connected account's is copied to the install's own files
| the way every picture is kept, and removed when it disconnects.
|
| A connection made before pictures were copied still names Meta's
| address. The first time it is listed its picture is asked for again
| and copied, or let go when Meta no longer gives one.
|
*/

package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/pictures"
)

/*
The side in pixels of a picture sent inline, twice the size it is shown
at, and how many pictures are read from Meta at once.
*/
const (
	socialPreviewSide    = 96
	socialPictureFetches = 6
)

/*
ownPicture reports whether the address is a picture in the install's own
files.
*/
func ownPicture(address string) bool {
	return strings.HasPrefix(address, "/avatars/")
}

/*
picture reads the picture at Meta's address, nothing when Meta does not
serve it.
*/
func (s *Social) picture(ctx context.Context, address string) []byte {
	if address == "" {
		return nil
	}

	fetched, err := s.graph("").Fetch(ctx, address, pictures.MaxBytes)
	if err != nil {
		s.Log.WarnContext(ctx, "social picture not read", log.String("platform", string(s.Platform)), log.Err(err))

		return nil
	}

	return fetched.Body
}

/*
inlinePictures swaps each account's picture address at Meta for the
picture itself, small, as a data address, and for nothing when it cannot
be read.
*/
func (s *Social) inlinePictures(ctx context.Context, accounts []SocialAccount) {
	var wg sync.WaitGroup

	turns := make(chan struct{}, socialPictureFetches)

	for i := range accounts {
		wg.Go(func() {
			turns <- struct{}{}

			defer func() { <-turns }()

			inline, err := pictures.Inline(bytes.NewReader(s.picture(ctx, accounts[i].PictureURL)), socialPreviewSide)
			if err != nil {
				inline = ""
			}

			accounts[i].PictureURL = inline
		})
	}

	wg.Wait()
}

/*
keepPicture copies the picture at Meta's address to the install's files
and returns its address there, nothing when it cannot be read or kept.
*/
func (s *Social) keepPicture(ctx context.Context, address string) string {
	body := s.picture(ctx, address)
	if len(body) == 0 || s.Pictures == nil {
		return ""
	}

	path, err := pictures.Store(ctx, s.Pictures, bytes.NewReader(body))
	if err != nil {
		s.Log.WarnContext(ctx, "social picture not kept", log.String("platform", string(s.Platform)), log.Err(err))

		return ""
	}

	return "/" + path
}

/*
dropPicture queues the removal of a picture kept in the install's files.
*/
func (s *Social) dropPicture(ctx context.Context, address string) {
	if !ownPicture(address) {
		return
	}

	if err := s.Queue.Push(ctx, pictures.DeleteArgs{Path: strings.TrimPrefix(address, "/")}); err != nil {
		s.Log.WarnContext(ctx, "social picture removal not queued",
			log.String("platform", string(s.Platform)), log.Err(err))
	}
}

/*
pictureAt asks Meta where the account's picture is now.
*/
func (s *Social) pictureAt(ctx context.Context, token, account string) string {
	fields := "picture{url}"
	if s.Platform == social.Instagram {
		fields = "profile_picture_url"
	}

	var out struct {
		Avatar  string `json:"profile_picture_url"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}

	if err := s.graph(token).Get(ctx, account, url.Values{"fields": {fields}}, &out); err != nil {
		return ""
	}

	return firstOf(out.Avatar, out.Picture.Data.URL)
}

/*
adoptPictures gives each connection that still names a picture at Meta a
copy of it in the install's files, or no picture when Meta gives none,
once.
*/
func (s *Social) adoptPictures(ctx context.Context, conns []models.ChannelConnection) {
	for i := range conns {
		facts := SocialFactsOf(conns[i])
		if facts.PictureURL == "" || ownPicture(facts.PictureURL) {
			continue
		}

		kept := ""
		if token, err := s.Token(ctx, conns[i]); err == nil {
			kept = s.keepPicture(ctx, s.pictureAt(ctx, token, facts.AccountID))
		}

		if config, err := s.repicture(ctx, conns[i], kept); err == nil {
			conns[i].Config = config
		} else {
			s.dropPicture(ctx, kept)
		}
	}
}

/*
repicture records the connection's picture and returns its config as it
now is.
*/
func (s *Social) repicture(ctx context.Context, conn models.ChannelConnection, address string) ([]byte, error) {
	patch, err := json.Marshal(map[string]string{"picture_url": address})
	if err != nil {
		return nil, err
	}

	err = s.read(ctx, func(q *models.Queries) error {
		return q.PatchChannelConnectionConfig(ctx, models.PatchChannelConnectionConfigParams{
			Patch: patch, Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
	})
	if err != nil {
		return nil, err
	}

	config := map[string]any{}
	_ = json.Unmarshal(conn.Config, &config)
	config["picture_url"] = address

	return json.Marshal(config)
}
