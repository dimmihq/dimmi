/*
|--------------------------------------------------------------------------
| Help Center Passes
|--------------------------------------------------------------------------
|
| A reader signed in to the help center is someone the workspace already
| knows, so the messenger on its pages signs them in too, without the
| team's identity secret. The help center hands the messenger a pass:
| shaped like an identity token, so the messenger reads whose it is the
| way it reads any token, but issued here, kept only as a hash in the
| cache, good for a minute and accepted once. It names the reader's own
| contact, never an id from the team's product, so it writes nothing
| over the customer and never claims an id their product might use.
|
*/

package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/models"
)

/*
passLifetime is how long a pass waits to be used.
*/
const passLifetime = time.Minute

/*
passType marks a pass's header, which is how a pass is told apart from
the team's own tokens.
*/
const passType = "dimmi-pass"

/*
ErrPassInvalid refuses a pass that is unknown, expired, used, or from
another workspace.
*/
var ErrPassInvalid = tokenError("pass_invalid",
	"The help center's sign-in pass is unknown, expired or used.", "The page asks for a new one.")

/*
Passes issues and redeems help center passes. Cache keeps each pass's
hash until it is used or expires.
*/
type Passes struct {
	Cache *cache.Store
}

/*
passHolder is who a pass signs in, as the cache keeps it.
*/
type passHolder struct {
	Workspace ids.UUID `json:"workspace"`
	Contact   ids.UUID `json:"contact"`
}

/*
Issue returns a pass that signs the workspace's contact in to its
messenger once, within a minute.
*/
func (p *Passes) Issue(ctx context.Context, workspaceID, contactID ids.UUID) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "none", "typ": passType})
	if err != nil {
		return "", fmt.Errorf("encode pass header: %w", err)
	}
	payload, err := json.Marshal(map[string]string{"sub": models.PublicID(models.PrefixContact, contactID)})
	if err != nil {
		return "", fmt.Errorf("encode pass payload: %w", err)
	}
	token := segment(header) + "." + segment(payload) + "." + str.Random(43)
	held, err := json.Marshal(passHolder{Workspace: workspaceID, Contact: contactID})
	if err != nil {
		return "", fmt.Errorf("encode pass holder: %w", err)
	}
	if err := p.Cache.Set(ctx, passKey(token), held, passLifetime); err != nil {
		return "", fmt.Errorf("keep pass: %w", err)
	}
	return token, nil
}

/*
Redeem returns the contact the pass signs in, once, and only in the
workspace it was issued for.
*/
func (p *Passes) Redeem(ctx context.Context, workspaceID ids.UUID, token string) (ids.UUID, error) {
	raw, err := p.Cache.Get(ctx, passKey(token))
	if err != nil || raw == nil {
		return ids.UUID{}, ErrPassInvalid
	}
	var held passHolder
	if json.Unmarshal(raw, &held) != nil || held.Workspace != workspaceID {
		return ids.UUID{}, ErrPassInvalid
	}
	first, err := p.Cache.Add(ctx, passKey(token)+":spent", []byte{1}, passLifetime)
	if err != nil {
		return ids.UUID{}, fmt.Errorf("spend pass: %w", err)
	}
	if !first {
		return ids.UUID{}, ErrPassInvalid
	}
	return held.Contact, nil
}

/*
IsPass reports whether a token is a help center pass rather than one the
team's product signed.
*/
func IsPass(token string) bool {
	head, _, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(head)
	if err != nil {
		return false
	}
	var header struct {
		Typ string `json:"typ"`
	}
	return json.Unmarshal(raw, &header) == nil && header.Typ == passType
}

func segment(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func passKey(token string) string {
	return "identity:pass:" + hashing.SHA256Hex([]byte(token))
}
