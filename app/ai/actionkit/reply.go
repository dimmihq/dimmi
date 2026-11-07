/*
|--------------------------------------------------------------------------
| What The Model Reads
|--------------------------------------------------------------------------
|
| A call's result reaches the model wrapped as data from the workspace's
| systems, with every < and > escaped so nothing inside can pose as an
| instruction or close the wrapper early. A call that waits says so in
| plain terms: nothing has run yet, who decides next, and that the
| customer will hear back here.
|
*/

package actionkit

import (
	"encoding/json"
	"strings"

	"github.com/dimmihq/dimmi/app/models"
)

/*
escape makes data inert inside the wrapper.
*/
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

/*
wrap puts a result in its untrusted-data wrapper.
*/
func wrap(s Spec, call models.AiActionCall, result json.RawMessage) string {
	return `<action_result action="` + s.Name + `" request_id="` +
		models.PublicID(models.PrefixAiActionCall, call.ID) + `">` + escape(string(result)) + `</action_result>`
}

/*
refusal is a call refused before it was recorded: bad arguments, or an
identity the context cannot prove.
*/
func refusal(s Spec, code, message string) CallReply {
	data, _ := json.Marshal(map[string]string{"error": code, "message": message})
	return CallReply{Text: `<action_result action="` + s.Name + `">` + escape(string(data)) + `</action_result>`}
}

/*
pendingText tells the model a write is waiting and on whom.
*/
func pendingText(s Spec, call models.AiActionCall) string {
	status, next := "pending_approval", "A teammate must approve this request before anything runs. "+
		"Nothing has run yet. Tell the customer what you requested, that the team will review it, "+
		"and that you will update them here."
	if call.State == "awaiting_confirmation" {
		status, next = "awaiting_customer_confirmation", "The customer now sees a confirmation card with this "+
			"summary and must press Confirm. Nothing has run yet. Ask them briefly to check the details "+
			"and confirm; do not ask them to type yes."
		if call.Approval != ApprovalNone {
			next += " After they confirm, a teammate still has to approve it."
		}
	}
	data, _ := json.Marshal(map[string]string{
		"status":     status,
		"request_id": models.PublicID(models.PrefixAiActionCall, call.ID),
		"summary":    call.Summary,
		"note":       next,
	})
	return `<action_result action="` + s.Name + `">` + escape(string(data)) + `</action_result>`
}
