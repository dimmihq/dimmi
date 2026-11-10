/*
|--------------------------------------------------------------------------
| Model Keys
|--------------------------------------------------------------------------
|
| A vector is only comparable with vectors of the same provider, model
| and size, so those three name the vector space:
| "<provider>:<model>@<dims>". The key id is not part of it: rotating a
| key, or moving to another key of the same model, keeps the vectors.
|
*/

package knowledge

import (
	"errors"
	"strconv"
	"strings"
)

/*
ErrModelKey is a model key that does not read as provider, model and
dimensions.
*/
var ErrModelKey = errors.New("knowledge: invalid model key")

/*
ModelKey is one vector space.
*/
type ModelKey struct {
	Provider string
	Model    string
	Dims     int32
}

/*
String writes the key.
*/
func (k ModelKey) String() string {
	return k.Provider + ":" + k.Model + "@" + strconv.Itoa(int(k.Dims))
}

/*
ParseModelKey reads a key; the model may itself hold colons and
slashes, the size follows the last "@".
*/
func ParseModelKey(s string) (ModelKey, error) {
	provider, rest, ok := strings.Cut(s, ":")
	at := strings.LastIndex(rest, "@")
	if !ok || provider == "" || at <= 0 {
		return ModelKey{}, ErrModelKey
	}
	dims, err := strconv.ParseInt(rest[at+1:], 10, 32)
	if err != nil || dims <= 0 {
		return ModelKey{}, ErrModelKey
	}
	return ModelKey{Provider: provider, Model: rest[:at], Dims: int32(dims)}, nil
}

/*
modelKeyOf reads a key, ok false when it does not read.
*/
func modelKeyOf(s string) (ModelKey, bool) {
	k, err := ParseModelKey(s)
	return k, err == nil
}
