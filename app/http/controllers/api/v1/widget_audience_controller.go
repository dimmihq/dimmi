/*
|--------------------------------------------------------------------------
| Messenger Rule Controller
|--------------------------------------------------------------------------
|
| PATCH /widget-settings/{widget}/audiences changes who sees a
| messenger's launcher, who may start a conversation in it and who sees
| each tab: a rule sent replaces that one, null removes it, and one left
| out stays. It answers the rules as stored.
|
*/

package v1

import (
	"context"
	"encoding/json"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetAudienceController saves messenger rules.
*/
type WidgetAudienceController struct {
	Surfaces *widget.Surfaces
	IDs      *ids.Registry
}

/*
MessengerAudiences is a messenger's rules as stored.
*/
type MessengerAudiences struct {
	Object string          `json:"object"`
	Rules  json.RawMessage `json:"rules"`
}

/*
MessengerAudiencesOutput is a messenger's rules.
*/
type MessengerAudiencesOutput struct {
	Body fhttp.Resource[MessengerAudiences]
}

/*
Update changes the messenger's rules.
*/
func (c *WidgetAudienceController) Update(ctx context.Context, in *requests.SaveWidgetAudiencesRequest,
) (*MessengerAudiencesOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixWidget), in.Widget)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	raw, _ := json.Marshal(in.Body)
	saved, err := c.Surfaces.SaveRules(ctx, widget.SaveRulesInput{
		Membership: membership(ctx), WidgetID: id, Rules: raw,
	})
	if err != nil {
		return nil, err
	}

	out := MessengerAudiences{Object: "messenger_audiences", Rules: saved.Audiences}

	return &MessengerAudiencesOutput{Body: fhttp.NewResource(out)}, nil
}
