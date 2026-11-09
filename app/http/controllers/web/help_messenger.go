/*
|--------------------------------------------------------------------------
| Messenger Sign-In From The Help Center
|--------------------------------------------------------------------------
|
| The messenger on the help center's pages asks who is signed in, the
| way it asks any site that signs its visitors in. A signed-in reader
| gets a pass that signs the messenger in as them once, within a minute;
| anyone else gets none, which signs a signed-in messenger out. The
| answer is never cached, and other sites cannot read it.
|
*/

package web

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
MessengerIdentity answers the messenger's question of who is signed in:
a pass for a signed-in reader, none for anyone else.
*/
func (c *HelpCenterController) MessengerIdentity(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	answer := map[string]*string{"token": nil}
	if c.Public == nil || c.Passes == nil {
		fhttp.WriteJSON(w, fhttp.StatusOK, answer)
		return
	}
	snap, err := c.Public.Load(r.Context(), r.PathValue("center"))
	if err != nil {
		fhttp.WriteJSON(w, fhttp.StatusOK, answer)
		return
	}
	contact, _, signed := c.readerOf(r, snap.HelpCenter)
	if !signed {
		fhttp.WriteJSON(w, fhttp.StatusOK, answer)
		return
	}
	pass, err := c.Passes.Issue(r.Context(), snap.HelpCenter.WorkspaceID, contact)
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	answer["token"] = &pass
	fhttp.WriteJSON(w, fhttp.StatusOK, answer)
}
