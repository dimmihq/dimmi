/*
|--------------------------------------------------------------------------
| Rating Links
|--------------------------------------------------------------------------
|
| An email ask carries links to a rating page that works without
| signing in. Each names its workspace and ask and is signed, so nobody
| can rate a conversation that is not theirs by guessing, and the
| server needs no lookup across workspaces to read one. The link stays
| readable for ever; the ask itself stops taking changes after a week.
|
*/

package ratings

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrBadLink refuses a rating link this install did not sign.
*/
var ErrBadLink = errors.New("ratings: link not recognized")

/*
Faces returns the five faces a customer picks from, worst first.
*/
func Faces() []string {
	return []string{"😠", "🙁", "😐", "🙂", "😍"}
}

/*
Labels returns what each of the five faces means, worst first.
*/
func Labels() []string {
	return []string{"Terrible", "Bad", "OK", "Good", "Amazing"}
}

/*
Links writes and reads the rating page links. BaseURL is the install's
address and Key the secret the links are signed with.
*/
type Links struct {
	BaseURL string
	Key     []byte
}

/*
Ref is what a link names: the workspace and the ask.
*/
type Ref struct {
	WorkspaceID ids.UUID
	RatingID    ids.UUID
}

/*
Page returns the rating page of the ask, picking the score when one is
given.
*/
func (l Links) Page(row models.ConversationRating, score int) string {
	url := l.BaseURL + "/rate?t=" + l.Token(row.WorkspaceID, row.ID)
	if score > 0 {
		url += "&score=" + strconv.Itoa(score)
	}

	return url
}

/*
Token signs the workspace and ask.
*/
func (l Links) Token(workspace, rating ids.UUID) string {
	raw := make([]byte, 0, 32)
	raw = append(raw, workspace[:]...)
	raw = append(raw, rating[:]...)

	return base64.RawURLEncoding.EncodeToString(raw) + "." + l.sign(raw)
}

/*
Read checks a token and returns what it names.
*/
func (l Links) Read(token string) (Ref, error) {
	if len(token) < 2 || len(token) > 128 {
		return Ref{}, ErrBadLink
	}

	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return Ref{}, ErrBadLink
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(raw) != 32 || !hashing.Equal([]byte(sig), []byte(l.sign(raw))) {
		return Ref{}, ErrBadLink
	}

	var ref Ref
	copy(ref.WorkspaceID[:], raw[:16])
	copy(ref.RatingID[:], raw[16:])

	return ref, nil
}

func (l Links) sign(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(hashing.HMAC(l.Key, raw)[:16])
}
