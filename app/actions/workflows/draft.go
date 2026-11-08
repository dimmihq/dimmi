/*
|--------------------------------------------------------------------------
| Working Copies
|--------------------------------------------------------------------------
|
| A working copy is stored in canonical form with its hash and kind, so
| the list can show whether it differs from the live version and
| whether it speaks to customers without reading the tree. A new
| workflow starts from an empty definition of its trigger unless it
| brings its own; steps and paths without ids get fresh ones.
|
*/

package workflows

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
draft is a working copy ready to store.
*/
type draft struct {
	raw  json.RawMessage
	hash string
	kind string
}

/*
startingSpec is the definition a new workflow starts with.
*/
func startingSpec(in CreateInput) (workflowspec.Spec, error) {
	if len(in.Spec) == 0 {
		if _, ok := workflowspec.FindTrigger(in.Trigger); !ok {
			return workflowspec.Spec{}, ErrUnknownTrigger
		}

		return emptySpec(in.Trigger), nil
	}

	spec, err := workflowspec.Parse(in.Spec)
	if err != nil {
		return spec, invalid(err)
	}

	if spec.Trigger == "" {
		spec.Trigger = in.Trigger
	}

	if _, ok := workflowspec.FindTrigger(spec.Trigger); !ok {
		return spec, ErrUnknownTrigger
	}

	if in.Trigger != "" && in.Trigger != spec.Trigger {
		return spec, ErrTriggerImmutable
	}

	return spec, nil
}

/*
emptySpec is a definition of the trigger with no steps and its default
rules. A quiet trigger starts with a silence of its usual length, so
a new workflow is never wrong before anyone touched it.
*/
func emptySpec(trigger string) workflowspec.Spec {
	t, _ := workflowspec.FindTrigger(trigger)
	spec := workflowspec.Spec{
		Schema: workflowspec.SchemaVersion, Trigger: trigger, Schedule: workflowspec.Schedule{Mode: "any"},
		Reentry: workflowspec.Reentry{Mode: t.Default}, Steps: []workflowspec.Step{},
	}

	switch trigger {
	case workflowspec.TriggerCustomerQuiet:
		spec.TriggerSettings = json.RawMessage(`{"after":"1d"}`)
	case workflowspec.TriggerTeammateQuiet:
		spec.TriggerSettings = json.RawMessage(`{"after":"30m"}`)
	}

	return spec
}

/*
encodeDraft gives missing ids, then stores the canonical form.
*/
func encodeDraft(spec workflowspec.Spec) (draft, error) {
	if spec.Schema == 0 {
		spec.Schema = workflowspec.SchemaVersion
	}

	workflowspec.AssignIDs(&spec)

	raw, err := workflowspec.Canonical(spec)
	if err != nil {
		return draft{}, invalid(err)
	}

	if len(raw) > workflowspec.MaxBytes {
		return draft{}, invalid(&workflowspec.InvalidError{Errors: []workflowspec.Issue{{
			Code: workflowspec.CodeLimitExceeded, Message: "The definition is over 256 KB.",
		}}})
	}

	hash, err := workflowspec.Hash(spec)
	if err != nil {
		return draft{}, fmt.Errorf("hash workflow: %w", err)
	}

	return draft{raw: raw, hash: hash, kind: workflowspec.WorkflowKind(spec)}, nil
}

/*
decodeDraft reads a stored definition.
*/
func decodeDraft(raw json.RawMessage) (workflowspec.Spec, error) {
	spec, err := workflowspec.Parse(raw)
	if err != nil {
		return spec, fmt.Errorf("read stored workflow: %w", err)
	}

	return spec, nil
}

/*
invalid turns a definition problem into the error clients see, listing
every issue.
*/
func invalid(err error) error {
	var bad *workflowspec.InvalidError
	if !errors.As(err, &bad) {
		return fault.New(fault.Invalid, "workflow_invalid", err.Error())
	}

	return fault.New(fault.Invalid, "workflow_invalid", "The workflow cannot run as it is.").
		With("errors", bad.Errors)
}
