/*
|--------------------------------------------------------------------------
| Meta's Signup Popup
|--------------------------------------------------------------------------
|
| What finishing Meta's Embedded Signup takes on the server: the popup's
| code exchanged for a token of the customer's business, the account and
| number it shared found from the token when the popup did not say, and
| a number new to the Cloud API registered with a PIN of its own.
|
*/

package channels

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
exchange swaps the popup's code for a token of the customer's business.
*/
func (w *WhatsApp) exchange(ctx context.Context, code, redirect string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", exceptions.ErrWhatsAppSignupExpired
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}

	query := url.Values{"client_id": {w.Service.AppID}, "client_secret": {w.Service.AppSecret}, "code": {code}}
	if redirect != "" {
		if !strings.HasPrefix(redirect, strings.TrimRight(w.AppURL, "/")+"/") {
			return "", exceptions.ErrWhatsAppSignupExpired
		}

		query.Set("redirect_uri", redirect)
	}

	err := w.graph("").Get(ctx, "oauth/access_token", query, &out)
	if refused, ok := errors.AsType[*wadriver.GraphError](err); ok && refused.Status < 500 {
		return "", exceptions.ErrWhatsAppSignupExpired
	}

	if err != nil {
		return "", w.unavailable(err)
	}

	if out.AccessToken == "" {
		return "", exceptions.ErrWhatsAppSignupExpired
	}

	return out.AccessToken, nil
}

/*
granted finds the account and number a signup shared when Meta's popup
did not say: the accounts the token was granted, and the first of their
numbers not connected here yet.
*/
func (w *WhatsApp) granted(ctx context.Context, token, accountID string) (string, string, error) {
	var out struct {
		Data struct {
			Granular []struct {
				Scope   string   `json:"scope"`
				Targets []string `json:"target_ids"`
			} `json:"granular_scopes"`
		} `json:"data"`
	}

	if err := w.graph(token).Get(ctx, "debug_token", url.Values{"input_token": {token}}, &out); err != nil {
		return "", "", w.unavailable(err)
	}

	accounts := []string{}
	if accountID != "" {
		accounts = append(accounts, accountID)
	}

	for _, g := range out.Data.Granular {
		if g.Scope == "whatsapp_business_management" {
			accounts = append(accounts, g.Targets...)
		}
	}

	connected, err := w.connectedNumbers(ctx, tenantOf(ctx))
	if err != nil {
		return "", "", err
	}

	for _, id := range accounts {
		account, err := w.readAccount(ctx, w.graph(token), id)
		if err != nil {
			continue
		}

		for _, n := range account.Numbers {
			if !slices.Contains(connected, n.ID) {
				return account.ID, n.ID, nil
			}
		}
	}

	return "", "", exceptions.ErrWhatsAppNumberNotFound
}

/*
tenantOf reads the workspace the context is scoped to.
*/
func tenantOf(ctx context.Context) ids.UUID {
	tenant, _ := tenancy.Required(ctx)
	id, _ := ids.Parse(tenant)

	return id
}

/*
register puts a number new to the Cloud API on it, with the PIN of its
two-step verification. A number already registered is left as it is.
*/
func (w *WhatsApp) register(ctx context.Context, graph *wadriver.Graph, phoneID, pin string) error {
	err := graph.Post(ctx, phoneID+"/register", map[string]any{"messaging_product": "whatsapp", "pin": pin}, nil)
	if err == nil {
		return nil
	}

	if refused, ok := errors.AsType[*wadriver.GraphError](err); ok && refused.Status < 500 {
		w.Log.WarnContext(ctx, "whatsapp number not registered", log.Int("code", refused.Code))

		return nil
	}

	return w.unavailable(err)
}
