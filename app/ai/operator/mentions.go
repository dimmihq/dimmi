/*
|--------------------------------------------------------------------------
| Mentions In A Routine's Instructions
|--------------------------------------------------------------------------
|
| A routine's instructions are plain words, and may also name things of
| the workspace as chips: a team, a teammate, an inbox, a tag, an AI
| agent, an article, a procedure, a ticket type, an action or a customer
| detail. A chip is kept in the text as [[kind:ref]], the way agent
| guidance and procedure steps keep theirs, so renaming what it points
| at never breaks the instructions. Operator reads each chip spelled out
| by the name its target has now, with its id where a tool takes one; a
| chip whose target is gone reads as something that no longer exists.
|
*/

package operator

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of thing instructions can mention. The first seven are the
kinds agent guidance and procedure steps share; the last three only a
routine names.
*/
const (
	MentionAction     = "action"
	MentionAttribute  = "attribute"
	MentionArticle    = "article"
	MentionTeam       = "handoff"
	MentionTag        = "tag"
	MentionTicketType = "ticket_type"
	MentionProcedure  = "sub_procedure"
	MentionTeammate   = "teammate"
	MentionInbox      = "inbox"
	MentionAgent      = "agent"
)

/*
mentionPattern matches one chip in a routine's instructions.
*/
var mentionPattern = regexp.MustCompile(`\[\[([a-z_]+):([^\]\s]+)\]\]`)

/*
mention is one chip and where it sits in the text.
*/
type mention struct {
	kind, ref  string
	start, end int
}

/*
mentioned is how one kind of chip is looked up and written out: the
prefix of its id, the query that names it, the words around the name,
and whether its id is added, for the kinds a tool takes one for.
*/
type mentioned struct {
	prefix models.Prefix
	query  string
	before string
	after  string
	withID bool
}

/*
mentionKinds says how each kind that points at a row is spelled out.
*/
func mentionKinds() map[string]mentioned {
	return map[string]mentioned{
		MentionTeam: {
			prefix: models.PrefixTeam, before: "the ", after: " team", withID: true,
			query: `SELECT name FROM teams WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		},
		MentionTag: {
			prefix: models.PrefixTag, before: `the tag "`, after: `"`,
			query: `SELECT name FROM tags WHERE workspace_id = $1 AND id = $2`,
		},
		MentionInbox: {
			prefix: models.PrefixInbox, before: "the ", after: " inbox",
			query: `SELECT name FROM inboxes WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		},
		MentionArticle: {
			prefix: models.PrefixArticle, before: `the help article "`, after: `"`, withID: true,
			query: `SELECT title FROM articles WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		},
		MentionProcedure: {
			prefix: models.PrefixProcedure, before: `the "`, after: `" procedure`, withID: true,
			query: `SELECT name FROM procedures WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		},
		MentionTicketType: {
			prefix: models.PrefixTicketType, before: `"`, after: `" tickets`,
			query: `SELECT name FROM ticket_types WHERE workspace_id = $1 AND id = $2`,
		},
		MentionAgent: {
			prefix: models.PrefixAiAgent, before: `the AI agent "`, after: `"`, withID: true,
			query: `SELECT name FROM ai_agents WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		},
		MentionAction: {
			prefix: models.PrefixAiAction, before: `the "`, after: `" action`,
			query: `SELECT coalesce(nullif(title, ''), name) FROM ai_actions
				WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL`,
		},
		MentionTeammate: {
			prefix: models.PrefixMember, withID: true,
			query: `SELECT coalesce(nullif(u.name, ''), u.email::text) FROM workspace_members m
				JOIN users u ON u.id = m.user_id WHERE m.workspace_id = $1 AND m.id = $2`,
		},
	}
}

/*
mentionsIn lists the chips of a text, in order.
*/
func mentionsIn(text string) []mention {
	var out []mention

	for _, m := range mentionPattern.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, mention{kind: text[m[2]:m[3]], ref: text[m[4]:m[5]], start: m[0], end: m[1]})
	}

	return out
}

/*
gone is what a chip reads as when its target cannot be found.
*/
const gone = "something that no longer exists"

/*
spellOut returns the text with every chip written out by the name its
target has now. Text without chips comes back untouched and reads
nothing.
*/
func (e *Engine) spellOut(ctx context.Context, workspace ids.UUID, text string) (string, error) {
	found := mentionsIn(text)
	if len(found) == 0 {
		return text, nil
	}

	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for _, m := range slices.Backward(found) {
			words, err := spelled(ctx, tx, workspace, m)
			if err != nil {
				return err
			}

			text = text[:m.start] + words + text[m.end:]
		}

		return nil
	})

	return text, err
}

/*
spelled writes one chip out.
*/
func spelled(ctx context.Context, tx *database.Tx, workspace ids.UUID, m mention) (string, error) {
	if m.kind == MentionAttribute {
		return "the customer's " + m.ref, nil
	}

	kind, known := mentionKinds()[m.kind]
	if !known {
		return gone, nil
	}

	id, ok := decodeMention(kind.prefix, m.ref)
	if !ok {
		return gone, nil
	}

	var name string

	err := tx.QueryRow(ctx, kind.query, workspace, id).Scan(&name)
	if errors.Is(err, database.ErrNotFound) {
		return gone, nil
	}

	if err != nil {
		return "", fmt.Errorf("name mentioned %s: %w", m.kind, err)
	}

	words := kind.before + name + kind.after
	if kind.withID {
		words += " (" + m.ref + ")"
	}

	return words, nil
}

/*
decodeMention reads a chip's id of the kind expected.
*/
func decodeMention(prefix models.Prefix, public string) (ids.UUID, bool) {
	r := ids.NewRegistry()
	if r.Register(string(prefix), string(prefix)) != nil {
		return ids.UUID{}, false
	}

	id, err := r.Decode(string(prefix), public)

	return id, err == nil
}
