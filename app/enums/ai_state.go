/*
|--------------------------------------------------------------------------
| AI State
|--------------------------------------------------------------------------
|
| A conversation's AI state says who may speak next: nobody yet
| (off), the agent (active), or only humans ever again (handed off).
|
*/

package enums

/*
AIState names the agent's state inside one conversation.
*/
type AIState string

/*
The AI states.
*/
const (
	AIStateOff       AIState = "off"
	AIStateActive    AIState = "active"
	AIStateHandedOff AIState = "handed_off"
)
