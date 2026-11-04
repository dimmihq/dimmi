/*
|--------------------------------------------------------------------------
| Threading
|--------------------------------------------------------------------------
|
| A reply finds its conversation by what it carries, strongest first:
| the reply token in the address it was sent to, the message it answers
| (In-Reply-To), the thread it continues (References, newest first),
| mail that mentioned it before it arrived, and only then its subject.
| These helpers read those inputs; the lookups themselves run against
| the workspace's own threading index.
|
| A message without a Message-ID still needs a stable identity, so a
| redelivery is recognized: one is derived from what it says.
|
*/

package email

import (
	"encoding/hex"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
Bounds of threading input: how many referenced ids are looked up, how
much of a body identifies a message without a Message-ID, and how long
the derived keys are.
*/
const (
	maxReferenceLookups = 20
	dedupeBodyBytes     = 4 << 10
	dedupeKeyLength     = 32
	syntheticPrefix     = "dimmi-synth-"
)

/*
Identity is what identifies a received message: its Message-ID, or
when it has none, its sender, date, subject and body.
*/
type Identity struct {
	MessageID string
	From      string
	Date      string
	Subject   string
	Body      string
}

/*
DedupeKey returns the key that recognizes the message when it arrives
again: from its Message-ID, or from what it says when it has none.
*/
func DedupeKey(id Identity) string {
	source := id.MessageID
	if source == "" {
		source = SyntheticID(id)
	}

	return "email:" + hex.EncodeToString(hashing.SHA256([]byte(source)))[:dedupeKeyLength]
}

/*
SyntheticID stands in for a missing Message-ID in the threading index.
It is never sent anywhere.
*/
func SyntheticID(id Identity) string {
	body := id.Body[:min(len(id.Body), dedupeBodyBytes)]
	sum := hashing.SHA256([]byte(strings.Join([]string{id.From, id.Date, id.Subject, body}, "\x00")))

	return syntheticPrefix + hex.EncodeToString(sum)[:dedupeKeyLength]
}

/*
ReferenceLookups returns the referenced ids worth looking up, newest
first and without those already in In-Reply-To.
*/
func ReferenceLookups(inReplyTo, references []string) []string {
	ids := slices.Clone(references)
	slices.Reverse(ids)
	ids = slices.DeleteFunc(ids, func(id string) bool { return slices.Contains(inReplyTo, id) })

	return ids[:min(len(ids), maxReferenceLookups)]
}

/*
ThreadRoot returns the id a new thread is keyed by: the first reference,
else what the message answers, else its own id.
*/
func ThreadRoot(references, inReplyTo []string, own string) string {
	switch {
	case len(references) > 0:
		return references[0]
	case len(inReplyTo) > 0:
		return inReplyTo[0]
	}

	return own
}
