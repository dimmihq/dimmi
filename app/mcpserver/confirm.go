/*
|--------------------------------------------------------------------------
| The Person Approves
|--------------------------------------------------------------------------
|
| An agent can be steered by text it reads, so for anything that reaches
| customers, reads private data or is hard to undo, its own judgement is
| not enough: the person approves each call. A client that can show a
| form asks the person directly. Any other client gets a preview of the
| call and a token instead, sealed with the app key to that tool, those
| exact arguments and that caller, good for ten minutes; the agent shows
| the person what it is about to do and calls again with the token only
| once they agree.
|
*/

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
approvalTTL is how long a confirmation token stays good.
*/
const approvalTTL = 10 * time.Minute

/*
approvalQuestion names the person's answer in a retried call.
*/
const approvalQuestion = "approve"

/*
seal is what a confirmation token holds: the call it approves and when
it stops being good.
*/
type seal struct {
	Tool    string    `json:"t"`
	Args    string    `json:"a"`
	Caller  string    `json:"c"`
	Expires time.Time `json:"e"`
}

/*
approvals decides whether a call that needs the person's approval runs.
*/
type approvals struct {
	sealer encryption.Encrypter
	clock  clock.Clock
	titles map[string]string
}

/*
decide runs the call when the person approved it, asks them when the
client can, and otherwise answers with a preview and a token.
*/
func (p approvals) decide(ctx context.Context, c mcp.Confirmation) (*mcp.Unconfirmed, error) {
	args, _ := c.Input.(map[string]any)
	fingerprint := p.fingerprint(ctx, c.Tool, args)

	if answer, ok := c.Answers[approvalQuestion]; ok {
		if answer.Action == "accept" && answer.Content["approve"] == true && p.valid(c.State, fingerprint) {
			return nil, nil
		}

		return declined(c.Tool), nil
	}

	if token, ok := args[confirmArgument].(string); ok && token != "" {
		if p.valid(token, fingerprint) {
			return nil, nil
		}

		return nil, errInvalidApproval
	}

	token, expires, err := p.issue(fingerprint)
	if err != nil {
		return nil, err
	}

	summary := preview(p.titles[c.Tool], c.Tool, args)
	if c.CanAsk {
		return &mcp.Unconfirmed{State: token, Ask: &mcp.Question{
			ID:      approvalQuestion,
			Message: summary + "\n\nAllow this?",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"approve": map[string]any{"type": "boolean", "title": "Allow"}},
				"required":   []string{"approve"},
			},
		}}, nil
	}

	return &mcp.Unconfirmed{
		Message: "Not done yet: this needs the person's approval.\n\n" + summary + "\n\nShow the person exactly " +
			"what will happen and ask them to approve. Only if they say yes, call " + c.Tool + " again with the " +
			"same arguments plus " + confirmArgument + "=\"" + token + "\". The token works for 10 minutes and only " +
			"for these exact arguments. If they say no, do not call it again.",
		Result: map[string]any{
			"status": "approval_required", confirmArgument: token, "expires_at": expires.Format(time.RFC3339),
		},
	}, nil
}

/*
errInvalidApproval refuses a token that does not approve this call.
*/
var errInvalidApproval = approvalError{}

/*
approvalError is a confirmation token that is wrong, expired, or for
other arguments.
*/
type approvalError struct{}

/*
Error describes the refused token.
*/
func (approvalError) Error() string { return "invalid confirmation token" }

/*
ToolResult explains the refused token.
*/
func (approvalError) ToolResult() (string, any) {
	text := "Not done: the confirmation token is wrong, expired, or was issued for different arguments. Call the " +
		"tool again without " + confirmArgument + " to get a new preview, show it to the person, and use the new " +
		"token only if they approve."

	return text, map[string]any{"error": map[string]any{"code": "invalid_confirmation", "message": text}}
}

/*
declined answers a call the person turned down.
*/
func declined(tool string) *mcp.Unconfirmed {
	return &mcp.Unconfirmed{
		Message: "The person did not approve, so " + tool + " was not run. Do not try again unless they ask.",
		Result:  map[string]any{"status": "declined"},
	}
}

/*
fingerprint names one call: the tool, its arguments without the token,
and who makes it.
*/
func (p approvals) fingerprint(ctx context.Context, tool string, args map[string]any) seal {
	rest := maps.Clone(args)
	delete(rest, confirmArgument)

	data, _ := json.Marshal(rest)
	caller := ""

	if principal, ok := fauth.From(ctx); ok {
		caller = principal.ID().String()
	}

	return seal{Tool: tool, Args: fmt.Sprintf("%x", hashing.SHA256(data)), Caller: caller}
}

/*
issue seals a token approving the call for the next ten minutes.
*/
func (p approvals) issue(s seal) (string, time.Time, error) {
	s.Expires = p.clock.Now().Add(approvalTTL)

	data, err := json.Marshal(s)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("seal approval: %w", err)
	}

	token, err := p.sealer.EncryptString(string(data))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("seal approval: %w", err)
	}

	return token, s.Expires, nil
}

/*
valid reports whether a token approves this very call and is still good.
*/
func (p approvals) valid(token string, want seal) bool {
	plain, err := p.sealer.DecryptString(token)
	if err != nil {
		return false
	}

	var got seal
	if err := json.Unmarshal([]byte(plain), &got); err != nil {
		return false
	}

	return got.Tool == want.Tool && got.Args == want.Args && got.Caller == want.Caller &&
		p.clock.Now().Before(got.Expires)
}

/*
preview says what a call will do, in the person's terms.
*/
func preview(title, tool string, args map[string]any) string {
	name := title
	if name == "" {
		name = tool
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		if key != confirmArgument {
			keys = append(keys, key)
		}
	}

	slices.Sort(keys)

	lines := []string{"About to: " + name}
	for _, key := range keys {
		lines = append(lines, "- "+key+": "+clip(compact(args[key])))
	}

	return strings.Join(lines, "\n")
}
