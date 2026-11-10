/*
|--------------------------------------------------------------------------
| Public Links
|--------------------------------------------------------------------------
|
| Every broadcast email carries links that work without signing in: the
| preference page, the one-click unsubscribe, the open pixel and each
| counted link. Each names its workspace and recipient and is signed, so
| nobody can unsubscribe someone else or inflate a report by guessing,
| and the server needs no lookup across workspaces to read one.
|
*/

package broadcasts

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrBadLink refuses a public link that was not signed by this install.
*/
var ErrBadLink = errors.New("broadcasts: link not recognized")

/*
Links writes and reads the public links. BaseURL is the install's
address and Key the secret the links are signed with.
*/
type Links struct {
	BaseURL string
	Key     []byte
}

/*
Ref is what a public link names: the workspace, the recipient and, for
a counted link, which one.
*/
type Ref struct {
	WorkspaceID ids.UUID
	RecipientID ids.UUID
	N           int
}

/*
Unsubscribe returns the preference page of a recipient.
*/
func (l Links) Unsubscribe(r models.BroadcastRecipient) string {
	return l.BaseURL + "/b/u/" + l.token(r, 0)
}

/*
OneClick returns where a mail app posts a one-click unsubscribe.
*/
func (l Links) OneClick(r models.BroadcastRecipient) string {
	return l.BaseURL + "/b/u/" + l.token(r, 0) + "/one-click"
}

/*
Open returns the pixel that records an open.
*/
func (l Links) Open(r models.BroadcastRecipient) string {
	return l.BaseURL + "/b/o/" + l.token(r, 0) + ".gif"
}

/*
Click returns the counted way through to the broadcast's n-th link.
*/
func (l Links) Click(r models.BroadcastRecipient, n int) string {
	return l.BaseURL + "/b/c/" + l.token(r, n)
}

/*
Read checks a link's token and returns what it names.
*/
func (l Links) Read(token string) (Ref, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return Ref{}, ErrBadLink
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(raw) != 34 || !hashing.Equal([]byte(sig), []byte(l.sign(raw))) {
		return Ref{}, ErrBadLink
	}

	var ref Ref

	copy(ref.WorkspaceID[:], raw[:16])
	copy(ref.RecipientID[:], raw[16:32])
	ref.N = int(binary.BigEndian.Uint16(raw[32:]))

	return ref, nil
}

/*
token signs the recipient and link number.
*/
func (l Links) token(r models.BroadcastRecipient, n int) string {
	raw := make([]byte, 0, 34)
	raw = append(raw, r.WorkspaceID[:]...)
	raw = append(raw, r.ID[:]...)
	raw = binary.BigEndian.AppendUint16(raw, linkNumber(n))

	return base64.RawURLEncoding.EncodeToString(raw) + "." + l.sign(raw)
}

/*
sign is the link's signature, shortened to keep URLs readable.
*/
func (l Links) sign(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(hashing.HMAC(l.Key, raw)[:16])
}

/*
linkNumber fits a link number into the two bytes a token keeps for it.
*/
func linkNumber(n int) uint16 {
	switch {
	case n < 0:
		return 0
	case n > math.MaxUint16:
		return math.MaxUint16
	default:
		return uint16(n)
	}
}
