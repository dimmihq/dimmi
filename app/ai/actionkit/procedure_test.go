package actionkit

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/support/actionresult"
)

func live(name string) Live {
	return Live{ActionID: ids.New(), Spec: Spec{Name: name, Title: name, SideEffect: SideEffectRead}}
}

func names(lives []Live) []string {
	out := make([]string, len(lives))
	for i, l := range lives {
		out[i] = l.Spec.Name
	}
	return out
}

func TestPinnedActionsAreAlwaysOffered(t *testing.T) {
	t.Parallel()

	var others []Live
	for i := range 25 {
		others = append(others, live(fmt.Sprintf("a_%02d", i)))
	}
	pinned := []Live{live("z_refund"), live("z_lookup")}

	out := pin(pinned, others)

	require.Len(t, out, maxOffered, "pinned actions fit inside the usual cap")
	assert.Contains(t, names(out), "z_refund", "a pinned action is offered even past the alphabetical cut")
	assert.Contains(t, names(out), "z_lookup")
	assert.True(t, slices.IsSorted(names(out)), "the list stays sorted so the prompt stays cacheable")

	var many []Live
	for i := range 35 {
		many = append(many, live(fmt.Sprintf("p_%02d", i)))
	}
	assert.Len(t, pin(many, others), maxPinned, "pins raise the cap, up to maxPinned")
}

func TestMockedCallsNeverRunAndWaitForAgreement(t *testing.T) {
	t.Parallel()

	schema := json.RawMessage(`{"type":"object","required":["order_id"],` +
		`"properties":{"order_id":{"type":"string"}}}`)
	lookup := Live{ActionID: ids.New(), Spec: Spec{
		Name: "get_order", Title: "Check order", SideEffect: SideEffectRead, InputSchema: schema,
		Confirmation: ConfirmationNone, Approval: ApprovalNone,
		ResponseFields: []actionresult.ResponseField{{Name: "status", Path: "status"}},
	}}

	got := MockCall(MockInput{
		Live: lookup, Args: json.RawMessage(`{"order_id":"1042"}`),
		Mock: &Mock{Status: 200, Body: json.RawMessage(`{"status":"shipped","secret":"x"}`)},
	})
	assert.True(t, got.Succeeded)
	assert.JSONEq(t, `{"status":"shipped"}`, string(got.Result), "the mock is shaped like a real response")
	assert.Equal(t, "1042", got.Input["order_id"])

	missing := MockCall(MockInput{Live: lookup, Args: json.RawMessage(`{}`)})
	assert.False(t, missing.Succeeded)
	assert.Contains(t, missing.Text, "invalid_arguments", "arguments are checked as a real call would")

	failed := MockCall(MockInput{
		Live: lookup, Args: json.RawMessage(`{"order_id":"9"}`), Mock: &Mock{Status: 404},
	})
	assert.False(t, failed.Succeeded)
	assert.Contains(t, failed.Text, "not_found")

	refund := lookup
	refund.Spec.Name, refund.Spec.SideEffect, refund.Spec.Confirmation = "refund", SideEffectWrite, ConfirmationCustomer
	waiting := MockCall(MockInput{Live: refund, Args: json.RawMessage(`{"order_id":"1"}`)})
	assert.True(t, waiting.Awaiting, "a write the customer confirms waits")
	agreed := MockCall(MockInput{Live: refund, Args: json.RawMessage(`{"order_id":"1"}`), Agreed: true})
	assert.True(t, agreed.Succeeded, "and runs its mock once confirmed")
}
