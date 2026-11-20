/*
|--------------------------------------------------------------------------
| Messenger And Instagram Customers
|--------------------------------------------------------------------------
|
| Meta's deliveries name a customer only by an id, so who they are is
| asked for apart: a name and picture on Messenger, and on Instagram
| their username, how many follow them, whether Instagram verified them
| and whether they follow the account. Meta answers only for someone who
| wrote to the account, and its picture addresses stop working within
| days, so a profile is asked for when it is shown and remembered for a
| day. On Messenger, until Meta grants profile reading, the name comes
| from the Page's conversation with the customer. A customer Meta says
| nothing about has no profile, and is asked about again next time.
|
| A conversation's standing is when its customer last wrote and what
| Meta therefore still allows: anyone may reply, only a teammate may, or
| nobody may until the customer writes again.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
socialProfileTTL is how long a customer's profile is remembered.
*/
const socialProfileTTL = 24 * time.Hour

/*
SocialProfile is who a customer is on the platform. Followers,
Verified and Follows are Instagram's; Follows says they follow the
account.
*/
type SocialProfile struct {
	Name       string `json:"name,omitempty"`
	Username   string `json:"username,omitempty"`
	PictureURL string `json:"picture_url,omitempty"`
	Followers  int64  `json:"followers,omitempty"`
	Verified   bool   `json:"verified,omitempty"`
	Follows    bool   `json:"follows,omitempty"`
}

/*
Profile returns who the customer is on the connection's platform, empty
when Meta does not say.
*/
func (s *Social) Profile(ctx context.Context, conn models.ChannelConnection, customer string) SocialProfile {
	key := "ch:profile:" + string(s.Platform) + ":" + conn.ID.String() + ":" + customer

	var profile SocialProfile

	if raw, err := s.Cache.Get(ctx, key); err == nil && json.Unmarshal(raw, &profile) == nil {
		return profile
	}

	token, err := s.Token(ctx, conn)
	if err != nil {
		return profile
	}

	profile = s.ask(ctx, token, customer)
	if profile.Name == "" && profile.Username == "" && s.Platform == social.Facebook {
		profile.Name = s.participant(ctx, token, SocialFactsOf(conn).AccountID, customer)
	}

	if profile.Name == "" && profile.Username == "" {
		return profile
	}

	if raw, err := json.Marshal(profile); err == nil {
		_ = s.Cache.Set(ctx, key, raw, socialProfileTTL)
	}

	return profile
}

/*
participant reads the customer's name from the Page's conversation with
them: Meta gives a Page the names of the people it talks to there,
where reading their profile needs a feature Meta grants only after
review.
*/
func (s *Social) participant(ctx context.Context, token, page, customer string) string {
	var out struct {
		Data []struct {
			Participants struct {
				Data []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"data"`
			} `json:"participants"`
		} `json:"data"`
	}

	err := s.graph(token).Get(ctx, page+"/conversations", url.Values{
		"platform": {"messenger"}, "user_id": {customer}, "fields": {"participants"},
	}, &out)
	if err != nil {
		return ""
	}

	for _, thread := range out.Data {
		for _, p := range thread.Participants.Data {
			if p.ID == customer {
				return cleanName(p.Name)
			}
		}
	}

	return ""
}

/*
ask reads the customer's profile from Meta.
*/
func (s *Social) ask(ctx context.Context, token, customer string) SocialProfile {
	fields := "first_name,last_name,profile_pic"
	if s.Platform == social.Instagram {
		fields = "name,username,profile_pic,follower_count,is_verified_user,is_user_follow_business"
	}

	var out struct {
		Name      string `json:"name"`
		First     string `json:"first_name"`
		Last      string `json:"last_name"`
		Username  string `json:"username"`
		Picture   string `json:"profile_pic"`
		Followers int64  `json:"follower_count"`
		Verified  bool   `json:"is_verified_user"`
		Follows   bool   `json:"is_user_follow_business"`
	}

	if err := s.graph(token).Get(ctx, customer, url.Values{"fields": {fields}}, &out); err != nil {
		return SocialProfile{}
	}

	return SocialProfile{
		Name:     cleanName(firstOf(out.Name, strings.TrimSpace(out.First+" "+out.Last))),
		Username: out.Username, PictureURL: out.Picture, Followers: out.Followers, Verified: out.Verified,
		Follows: out.Follows,
	}
}

/*
whereIs finds the conversation and its connection on the platform.
*/
func (s *Social) whereIs(ctx context.Context, ws, conversationID ids.UUID) (
	models.Conversation, models.ChannelConnection, error,
) {
	var (
		conv models.Conversation
		conn models.ChannelConnection
	)

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		conv, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conversationID})
		if err != nil {
			return err
		}

		conn, err = q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: conv.ChannelConnectionID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && conn.Type != string(s.Platform)) {
		return conv, conn, exceptions.ErrNotFound
	}

	return conv, conn, err
}

/*
SocialConversation is where a conversation stands on its platform: its
connection, when the customer last wrote, what Meta still allows, until
when a teammate may reply, and who the customer is.
*/
type SocialConversation struct {
	Connection models.ChannelConnection
	Phase      social.Phase
	LastWrote  time.Time
	OpenUntil  time.Time
	HumanUntil time.Time
	Profile    SocialProfile
}

/*
Conversation returns where the conversation stands on the platform.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound] for a
conversation that is not the workspace's or not on the platform.
*/
func (s *Social) Conversation(ctx context.Context, m authz.Membership, conversationID ids.UUID) (
	SocialConversation, error,
) {
	if !m.Can("conversations:read") {
		return SocialConversation{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var out SocialConversation

	conv, conn, err := s.whereIs(ctx, m.Workspace.ID, conversationID)
	if err != nil {
		return out, err
	}

	out.Connection = conn

	window, err := s.Driver.Places.Window(ctx, out.Connection, conv.ContactID)
	if err != nil {
		return out, err
	}

	out.Phase, out.LastWrote = s.Driver.PhaseAt(window, s.Clock.Now()), window.LastInboundAt
	if !window.LastInboundAt.IsZero() {
		out.OpenUntil = window.LastInboundAt.Add(s.Config.Window)
		out.HumanUntil = window.LastInboundAt.Add(s.Config.HumanWindow)
	}

	if conv.ExternalThreadKey != nil {
		if target, ok := social.ParseThreadKey(*conv.ExternalThreadKey); ok {
			out.Profile = s.Profile(ctx, out.Connection, target.Customer)
		}
	}

	return out, nil
}
