/*
|--------------------------------------------------------------------------
| Actions From Automations
|--------------------------------------------------------------------------
|
| Workflows and series call the same actions agents do, so a team
| connects a system once. An automation's call is the team's own doing,
| written into the workflow ahead of time, so it waits for no customer
| confirmation and no teammate approval: it runs the action's published
| version at once, with inputs the team mapped. Inputs are checked
| against the action's schema like a model's, and contact values bind
| only from what proves who the customer is, as they do for an agent.
| The call is recorded like any other, on the conversation's timeline
| when there is one, and the connection's health follows its outcome.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AutomationCall is one call an automation makes: the action, the
conversation and contact it is for when there are ones, the inputs as
the team mapped them, each still text, what proves who the customer
is, and the name the timeline gives the automation.
*/
type AutomationCall struct {
	WorkspaceID    ids.UUID
	ActionID       ids.UUID
	ConversationID *ids.UUID
	ContactID      *ids.UUID
	Inputs         map[string]string
	Identity       Identity
	By             string
}

/*
AutomationResult is how the call went: success or a stable error code,
the upstream status, and the result as the action shapes it.
*/
type AutomationResult struct {
	Succeeded  bool
	ErrorCode  string
	HTTPStatus int
	Result     json.RawMessage
}

/*
failed is a call that could not be made.
*/
func failed(code string) AutomationResult {
	return AutomationResult{ErrorCode: code}
}

/*
RunForAutomation runs the action's published version. A call that
cannot be made is a result with its code, never an error; an error is
a database failure the job retries.
*/
func (k *Kit) RunForAutomation(ctx context.Context, in AutomationCall) (AutomationResult, error) {
	t, code, err := k.automationTarget(ctx, in)
	if err != nil || code != "" {
		return failed(code), err
	}

	call, err := k.recordAutomationCall(ctx, in, t)
	if err != nil {
		return AutomationResult{}, err
	}

	t.target.CallID = call.ID
	out := k.Send(ctx, t.target)

	actor := conversations.Actor{Type: string(enums.AuthorTypeSystem), Name: in.By}
	if _, err := k.finish(ctx, finishing{call: call, out: out, actor: actor}); err != nil {
		return AutomationResult{}, err
	}

	return AutomationResult{
		Succeeded: out.Succeeded, ErrorCode: out.ErrorCode, HTTPStatus: out.HTTPStatus, Result: out.Result,
	}, nil
}

/*
automationTarget is an automation's call ready to send: the published
version it runs and the execution.
*/
type automationTarget struct {
	live   Live
	target Target
}

/*
automationTarget reads and checks everything the call needs, or the
code of why it cannot be made.
*/
func (k *Kit) automationTarget(ctx context.Context, in AutomationCall) (automationTarget, string, error) {
	found, code, err := k.liveAction(ctx, in.WorkspaceID, in.ActionID)
	if err != nil || code != "" {
		return automationTarget{}, code, err
	}

	live, conn := found.live, found.conn

	input, b, code := automationInput(live.Spec, in)
	if code != "" {
		return automationTarget{}, code, nil
	}

	creds, err := OpenCredentials(ctx, k.Keys, conn)
	if err != nil {
		return automationTarget{}, "credentials_unreadable", nil //nolint:nilerr // it fails the step
	}

	return automationTarget{live: live, target: Target{
		Conn: conn, Creds: creds, Spec: live.Spec, Input: input, Bindings: b, WorkspaceID: in.WorkspaceID,
	}}, "", nil
}

/*
automationInput checks the mapped inputs against the action's schema
and binds the contact values it uses, or the code of why it cannot.
*/
func automationInput(s Spec, in AutomationCall) (map[string]any, Bindings, string) {
	args, _ := json.Marshal(typedInputs(s, in.Inputs))

	input, refused := validate(s, args)
	if refused != nil {
		return nil, Bindings{}, "invalid_arguments"
	}

	contact := ContactValues(in.Identity)
	if s.UsesContact() && !bindable(s, contact) {
		return nil, Bindings{}, "identity_unavailable"
	}

	return input, bindingsFor(s, contact), ""
}

/*
liveCall is an action's published version, the action and its
connection.
*/
type liveCall struct {
	live   Live
	action models.AiAction
	conn   models.AiActionConnection
}

/*
liveAction reads the action's published version and its connection,
or the code of why it cannot run.
*/
func (k *Kit) liveAction(ctx context.Context, workspaceID, actionID ids.UUID) (liveCall, string, error) {
	var (
		out  liveCall
		code string
	)

	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error
		if out, code, err = publishedVersion(ctx, q, workspaceID, actionID); err != nil || code != "" {
			return err
		}

		out.conn, err = q.FindActionConnection(ctx, models.FindActionConnectionParams{
			WorkspaceID: workspaceID, ID: out.action.ConnectionID,
		})
		if errors.Is(err, database.ErrNotFound) || err == nil && out.conn.Status != "active" {
			code = "connection_disabled"
			return nil
		}

		if err != nil {
			return fmt.Errorf("find action connection: %w", err)
		}

		return nil
	})

	return out, code, err
}

/*
publishedVersion reads the action and the version it published, or
the code of an action that is not published.
*/
func publishedVersion(ctx context.Context, q *models.Queries, workspaceID, actionID ids.UUID) (
	liveCall, string, error,
) {
	a, err := q.FindAction(ctx, models.FindActionParams{WorkspaceID: workspaceID, ID: actionID})
	if errors.Is(err, database.ErrNotFound) {
		return liveCall{}, "action_unavailable", nil
	}

	if err != nil {
		return liveCall{}, "", fmt.Errorf("find action: %w", err)
	}

	if a.Status != "published" || a.PublishedVersionID == nil {
		return liveCall{}, "action_unavailable", nil
	}

	v, err := q.FindActionVersion(ctx, models.FindActionVersionParams{
		WorkspaceID: workspaceID, ID: *a.PublishedVersionID,
	})
	if err != nil {
		return liveCall{}, "", fmt.Errorf("find action version: %w", err)
	}

	s, err := FromVersion(v)

	return liveCall{live: Live{ActionID: a.ID, VersionID: v.ID, Spec: s}, action: a}, "", err
}

/*
recordAutomationCall records the call as running.
*/
func (k *Kit) recordAutomationCall(ctx context.Context, in AutomationCall, t automationTarget) (
	models.AiActionCall, error,
) {
	var call models.AiActionCall

	// The team wrote the call into the automation themselves, so it is
	// stored as one that waits for nobody.
	live := t.live
	live.Spec.Confirmation, live.Spec.Approval = ConfirmationNone, ApprovalNone

	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		call, err = k.insert(ctx, tx, CallInput{
			WorkspaceID: in.WorkspaceID, Live: live, ConversationID: derefID(in.ConversationID),
			ContactID: derefID(in.ContactID),
		}, newCall{input: t.target.Input, bindings: t.target.Bindings, state: "running", sent: true})

		return err
	})

	return call, err
}

/*
AutomationIdentity is what an automation knows proves who a contact
is: an external id the workspace set through its API or a verified
messenger, and an email the contact proved. A claimed email alone
binds nothing, as it binds nothing for an agent.
*/
func AutomationIdentity(c models.Contact) Identity {
	verified := c.ExternalID != nil && *c.ExternalID != ""

	return Identity{Verified: verified, EmailProven: c.EmailVerified, Contact: &c}
}

/*
derefID reads an optional id, the zero id for none.
*/
func derefID(id *ids.UUID) ids.UUID {
	if id == nil {
		return ids.UUID{}
	}

	return *id
}

/*
typedInputs turns the mapped text inputs into the types the action's
schema asks for: numbers, yes or no, and JSON for lists and objects.
An input left empty is left out, and one that does not read as its
type stays text, so the schema check refuses it instead of the call
guessing.
*/
func typedInputs(s Spec, inputs map[string]string) map[string]any {
	var schema struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	_ = json.Unmarshal(s.ModelSchema(), &schema)

	out := map[string]any{}

	for name, text := range inputs {
		if strings.TrimSpace(text) == "" {
			continue
		}

		out[name] = typed(typeOf(schema.Properties[name].Type), text)
	}

	return out
}

/*
typeOf reads a schema type, the first non-null one of a list.
*/
func typeOf(t any) string {
	switch v := t.(type) {
	case string:
		return v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "null" {
				return s
			}
		}
	}

	return ""
}

/*
typed reads one text input as the type.
*/
func typed(kind, text string) any {
	text = strings.TrimSpace(text)

	switch kind {
	case "integer":
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n
		}
	case "number":
		if n, err := strconv.ParseFloat(text, 64); err == nil {
			return n
		}
	case "boolean":
		if b, err := strconv.ParseBool(text); err == nil {
			return b
		}
	case "array", "object":
		var v any
		if json.Unmarshal([]byte(text), &v) == nil {
			return v
		}
	}

	return text
}
