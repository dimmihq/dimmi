/*
|--------------------------------------------------------------------------
| Checking A Page Or Instagram Account
|--------------------------------------------------------------------------
|
| A connected account that receives nothing looks the same as one nobody
| wrote to, so the channel can ask Meta what stands in the way: whether
| Meta still takes the account's token, whether Meta delivers its events
| to this install, and whether the account lets apps read its messages,
| which on Instagram is a setting its owner turns on. Each answer is
| Meta's own, with its words when it refuses.
|
*/

package channels

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
SocialFinding is one thing the check asked Meta: what was asked, whether
it holds, and what Meta said when it does not.
*/
type SocialFinding struct {
	Check  string
	OK     bool
	Detail string
}

/*
tokenRefused is Meta's code for a token it no longer takes.
*/
const tokenRefused = 190

/*
The checks, in the order they are asked.
*/
const (
	SocialCheckToken    = "token"
	SocialCheckDelivery = "delivery"
	SocialCheckMessages = "messages"
)

/*
Check asks Meta what stands between the connection and messages that
arrive.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (s *Social) Check(ctx context.Context, m authz.Membership, id ids.UUID) ([]SocialFinding, error) {
	conn, err := s.Find(ctx, m, id)
	if err != nil {
		return nil, err
	}

	if !m.Can("channels:manage") {
		return nil, exceptions.ErrForbidden
	}

	secrets, err := s.open(ctx, conn)
	if err != nil {
		return nil, err
	}

	if secrets.AccessToken == "" {
		return []SocialFinding{{Check: SocialCheckToken, Detail: "The connection has no token. Connect it again."}}, nil
	}

	graph, account := s.graph(secrets.AccessToken), SocialFactsOf(conn).AccountID

	delivery, tokenDead := s.delivering(ctx, graph, account)
	if tokenDead {
		return []SocialFinding{{Check: SocialCheckToken, Detail: delivery.Detail}}, nil
	}

	return []SocialFinding{{Check: SocialCheckToken, OK: true}, delivery, s.readable(ctx, graph, account)}, nil
}

/*
delivering asks whether Meta delivers the account's events to this
install, and reports a token Meta no longer takes.
*/
func (s *Social) delivering(ctx context.Context, graph *meta.Graph, account string) (SocialFinding, bool) {
	var subscribed struct {
		Data []struct {
			ID     string   `json:"id"`
			Fields []string `json:"subscribed_fields"`
		} `json:"data"`
	}

	err := graph.Get(ctx, account+"/subscribed_apps", nil, &subscribed)
	if err != nil {
		refused, isMeta := errors.AsType[*meta.GraphError](err)

		return SocialFinding{Check: SocialCheckDelivery, Detail: said(err)}, isMeta && refused.Code == tokenRefused
	}

	for _, app := range subscribed.Data {
		if app.ID == s.App.AppID || len(subscribed.Data) == 1 {
			return SocialFinding{Check: SocialCheckDelivery, OK: true, Detail: strings.Join(app.Fields, ", ")}, false
		}
	}

	return SocialFinding{
		Check: SocialCheckDelivery, Detail: "Meta is not delivering this account's events here. Connect it again.",
	}, false
}

/*
readable asks whether the account lets this install read its messages,
and says when its newest conversation last changed.
*/
func (s *Social) readable(ctx context.Context, graph *meta.Graph, account string) SocialFinding {
	query := url.Values{"limit": {"1"}, "fields": {"id,updated_time"}}
	if s.Platform == social.Instagram {
		query.Set("platform", "instagram")
	}

	var threads struct {
		Data []struct {
			Updated string `json:"updated_time"`
		} `json:"data"`
	}

	err := graph.Get(ctx, account+"/conversations", query, &threads)

	found := SocialFinding{Check: SocialCheckMessages, OK: err == nil, Detail: said(err)}
	if err == nil && len(threads.Data) > 0 {
		found.Detail = "Newest conversation at Meta changed " + threads.Data[0].Updated
	}

	return found
}

/*
said is what Meta said when it refused, empty when it did not.
*/
func said(err error) string {
	if err == nil {
		return ""
	}

	if refused, ok := errors.AsType[*meta.GraphError](err); ok {
		return strings.TrimSpace(refused.Message)
	}

	return "Meta could not be reached."
}
