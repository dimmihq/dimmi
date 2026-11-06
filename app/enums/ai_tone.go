/*
|--------------------------------------------------------------------------
| AI Tone
|--------------------------------------------------------------------------
|
| The workspace picks how the agent sounds: a named tone maps to one
| tuned prompt file, so every reply on every channel reads the same
| way.
|
*/

package enums

/*
AITone names one way the agent may sound.
*/
type AITone string

/*
The tones.
*/
const (
	AIToneFriendly     AITone = "friendly"
	AIToneProfessional AITone = "professional"
	AIToneConcise      AITone = "concise"
)

/*
AITones returns every tone.
*/
func AITones() []AITone {
	return []AITone{AIToneFriendly, AIToneProfessional, AIToneConcise}
}
