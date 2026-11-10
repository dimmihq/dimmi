/*
|--------------------------------------------------------------------------
| Integration OAuth Callback Controller
|--------------------------------------------------------------------------
|
| GET /integrations/{provider}/callback is where a provider sends the
| browser back after an install or a teammate's grant. It only counts in
| the browser session that started the attempt. The browser lands on
| the page it came from, with the installation it connected or the code
| of what went wrong; a callback that cannot count at all, expired,
| reused or opened elsewhere, shows a page asking to start again.
|
*/

package web

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/session"

	"github.com/dimmihq/dimmi/app/actions/integrations"
)

/*
IntegrationOAuthCallbackController finishes installs and grants.
*/
type IntegrationOAuthCallbackController struct {
	Complete *integrations.CompleteCallback
	Errors   fhttp.Errors
}

/*
Invoke finishes the attempt and sends the browser on.
*/
func (c *IntegrationOAuthCallbackController) Invoke(w fhttp.ResponseWriter, r *fhttp.Request) {
	var binding string
	if s, ok := session.From(r.Context()); ok {
		binding = s.ID().String()
	}

	result, err := c.Complete.Handle(r.Context(), integrations.CallbackInput{
		Provider: r.PathValue("provider"), Binding: binding, Query: r.URL.Query(),
	})
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	redirect(w, result.Location())
}
