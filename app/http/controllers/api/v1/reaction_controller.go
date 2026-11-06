/*
|--------------------------------------------------------------------------
| Reaction Controller
|--------------------------------------------------------------------------
|
| A teammate's emoji reactions to notes: adding one and taking it off.
| Both answer with the note as it now is, its reactions included, and
| doing either twice is answered the same way.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReactionController adds and removes the caller's reactions to notes.
*/
type ReactionController struct {
	React *messages.ReactToNote
	IDs   *ids.Registry
}

/*
Store adds the caller's emoji to the note.
*/
func (c *ReactionController) Store(ctx context.Context, in *requests.AddReactionRequest) (*EntryOutput, error) {
	return c.change(ctx, reaction{in.Conversation, in.Message, in.Body.Emoji}, false)
}

/*
Destroy takes the caller's emoji off the note.
*/
func (c *ReactionController) Destroy(ctx context.Context, in *requests.RemoveReactionRequest) (*EntryOutput, error) {
	return c.change(ctx, reaction{in.Conversation, in.Message, in.Emoji}, true)
}

/*
reaction names a note by its conversation and id, and the emoji.
*/
type reaction struct {
	conversation, message, emoji string
}

/*
change adds or removes the reaction and answers with the note.
*/
func (c *ReactionController) change(ctx context.Context, r reaction, remove bool) (*EntryOutput, error) {
	ref, err := conversationRef(c.IDs, r.conversation)
	if err != nil {
		return nil, err
	}

	id, err := c.IDs.Decode(string(models.PrefixMessage), r.message)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	written, err := c.React.Handle(ctx, messages.ReactInput{
		Membership: membership(ctx), Ref: ref, MessageID: id, Emoji: r.emoji, Remove: remove,
		MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	return &EntryOutput{Body: writtenOutput(written).Body}, nil
}
