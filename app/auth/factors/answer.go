/*
|--------------------------------------------------------------------------
| Answers and Prompts
|--------------------------------------------------------------------------
|
| A code typed by the person proves itself; a credential signed by a
| device proves only that it signed what the server asked for. Kinds of
| the second sort hand the browser a prompt before the person answers,
| keep its state on the server, and check the answer against it. They
| also activate differently: the first answer registers the credential
| instead of checking it. Kinds answered with codes need neither and
| simply leave both out.
|
*/

package factors

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Answer is what the person sent to prove they hold a factor: a typed
code, or the credential their device signed, with the state the kind
kept when its prompt began.
*/
type Answer struct {
	Code       string
	Credential json.RawMessage
	State      json.RawMessage
}

/*
Prompt is what a challenge needs from a kind before the person can
answer: the options to hand the browser and the state to keep until
the answer comes. Both are nil for kinds answered with a typed code.
*/
type Prompt struct {
	Options json.RawMessage
	State   json.RawMessage
}

/*
Activator is a kind whose first answer does more than a later one, such
as registering the credential it carries.
*/
type Activator interface {
	/*
		Activate checks the first answer of a pending factor and updates
		what it stores. It returns ErrInvalidCode or ErrCodeReused when the
		answer does not pass.
	*/
	Activate(ctx context.Context, factor *models.UserAuthFactor, answer Answer) error
}

/*
Prompter is a kind that hands the browser options before the person can
answer a challenge.
*/
type Prompter interface {
	/*
		Challenge returns what the person needs to answer with one of the
		offered factors of the kind.
	*/
	Challenge(ctx context.Context, user models.User, offered []models.UserAuthFactor) (Prompt, error)
}

/*
Activate checks the first answer of a pending factor of the kind: as
its Activator does, or as any later answer would.
*/
func Activate(ctx context.Context, driver Factor, factor *models.UserAuthFactor, answer Answer) error {
	if a, ok := driver.(Activator); ok {
		return a.Activate(ctx, factor, answer)
	}

	return driver.Verify(ctx, factor, answer)
}

/*
Challenge returns the prompt of the kind for the offered factors, empty
for kinds answered with a typed code.
*/
func Challenge(ctx context.Context, driver Factor, user models.User, offered []models.UserAuthFactor) (Prompt, error) {
	p, ok := driver.(Prompter)
	if !ok {
		return Prompt{}, nil
	}

	return p.Challenge(ctx, user, offered)
}
