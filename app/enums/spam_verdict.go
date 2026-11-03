/*
|--------------------------------------------------------------------------
| Spam Verdict
|--------------------------------------------------------------------------
|
| What a channel or its provider concluded about an inbound message
| before the inbox decides where it goes.
|
*/

package enums

/*
SpamVerdict is a channel's opinion of an inbound message.
*/
type SpamVerdict string

/*
The spam verdicts.
*/
const (
	SpamVerdictUnknown SpamVerdict = "unknown"
	SpamVerdictHam     SpamVerdict = "ham"
	SpamVerdictSpam    SpamVerdict = "spam"
)
