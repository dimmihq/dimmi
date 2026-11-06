/*
|--------------------------------------------------------------------------
| Mentionable Controller
|--------------------------------------------------------------------------
|
| Who a note on a conversation may mention, for the composer's at-sign
| suggestions: teammates with their availability and whether they see
| the conversation, and teams with how many active members they have.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MentionableMember is a teammate a note may mention.
*/
type MentionableMember struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	AvatarURL    *string `json:"avatar_url"`
	Availability string  `json:"availability"`
	CanSee       bool    `json:"can_see"`
}

/*
MentionableTeam is a team a note may mention.
*/
type MentionableTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MemberCount int64  `json:"member_count"`
}

/*
MentionablesBody is who a note may mention.
*/
type MentionablesBody struct {
	Object  string              `json:"object"`
	Members []MentionableMember `json:"members"`
	Teams   []MentionableTeam   `json:"teams"`
}

/*
MentionablesOutput answers with who a note may mention.
*/
type MentionablesOutput struct {
	Body fhttp.Resource[MentionablesBody]
}

/*
MentionableController lists who a note may mention.
*/
type MentionableController struct {
	List *conversations.ListMentionables
	IDs  *ids.Registry
}

/*
Index returns who a note on the conversation may mention.
*/
func (c *MentionableController) Index(
	ctx context.Context, in *requests.MentionablesRequest,
) (*MentionablesOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	found, err := c.List.Handle(ctx, conversations.MentionablesInput{
		Membership: membership(ctx), Ref: ref, Query: in.Query,
	})
	if err != nil {
		return nil, err
	}

	body := MentionablesBody{Object: "mentionables", Members: []MentionableMember{}, Teams: []MentionableTeam{}}
	for _, m := range found.Members {
		body.Members = append(body.Members, MentionableMember{
			ID: models.PublicID(models.PrefixMember, m.ID), Name: m.Name,
			AvatarURL: models.User{AvatarPath: m.AvatarPath}.AvatarURL(), Availability: m.Availability,
			CanSee: m.CanSee,
		})
	}

	for _, t := range found.Teams {
		body.Teams = append(body.Teams, MentionableTeam{
			ID: models.PublicID(models.PrefixTeam, t.ID), Name: t.Name, MemberCount: t.MemberCount,
		})
	}

	return &MentionablesOutput{Body: fhttp.NewResource(body)}, nil
}
