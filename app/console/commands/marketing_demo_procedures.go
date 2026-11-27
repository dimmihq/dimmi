//go:build dev

/*
|--------------------------------------------------------------------------
| Marketing Demo: Actions And Procedures
|--------------------------------------------------------------------------
|
| Seeds Lumen's billing API actions, two published versions of Refund a
| duplicate charge (using @ chips and a code-checked Rule branch), and
| three passing saved simulations for the Procedures page shots.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionresult"
	"github.com/dimmihq/dimmi/app/support/procedure"
)

/*
actions connects Lumen's billing API and publishes two actions with saved
test responses so the procedure editor shows real chips and rule fields.
*/
func (c lumenSeeder) actions(ctx context.Context, owner authz.Membership) (map[string]string, error) {
	a := c.App
	conn, err := a.Actions.AI.ActionConnections.Create(ctx, agentactions.ConnectionInput{
		Membership: owner, Name: new("Lumen Billing API"), Kind: "http",
		BaseURL: new("https://api.lumen.test/v1"), AuthType: new("bearer"),
		Token: new("sk_test_lumen_demo_not_real"), TimeoutMS: new(int32(8000)),
	})
	if err != nil {
		return nil, fmt.Errorf("create action connection: %w", err)
	}

	samples := map[string]map[string]any{
		"list_charges":  {"duplicate_count": 1, "amount": 49, "days_since_charge": 4},
		"refund_charge": {"refund_id": "re_01JCK8", "status": "pending_approval"},
	}

	published := map[string]string{}
	for _, def := range marketingActionDefs(owner, conn.ID) {
		made, err := a.Actions.AI.ActionDefinitions.Create(ctx, def)
		if err != nil {
			return nil, fmt.Errorf("create action %s: %w", *def.Name, err)
		}
		if _, err := a.Actions.AI.ActionDefinitions.Publish(ctx, owner, made.Action.ID, "Initial version"); err != nil {
			return nil, fmt.Errorf("publish action %s: %w", *def.Name, err)
		}
		published[*def.Name] = models.PublicID(models.PrefixAiAction, made.Action.ID)
		body, _ := json.Marshal(samples[*def.Name])
		if err := models.New(a.DB).SetActionLastTestResponse(ctx, models.SetActionLastTestResponseParams{
			WorkspaceID: owner.Workspace.ID, ID: made.Action.ID, Response: body,
		}); err != nil {
			return nil, fmt.Errorf("save test response %s: %w", *def.Name, err)
		}
	}

	return published, nil
}

/*
marketingActionDefs builds the two billing actions Lumen's procedure calls.
*/
func marketingActionDefs(owner authz.Membership, connID ids.UUID) []agentactions.DefinitionInput {
	schema := func(v any) *json.RawMessage {
		b, _ := json.Marshal(v)
		raw := json.RawMessage(b)
		return &raw
	}
	fields := func(pairs ...string) *[]actionresult.ResponseField {
		var out []actionresult.ResponseField
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, actionresult.ResponseField{Name: pairs[i], Path: pairs[i+1]})
		}
		return &out
	}
	channels := &[]string{"widget", "email", "api"}

	return []agentactions.DefinitionInput{
		{
			Membership: owner, ConnectionID: &connID, Name: new("list_charges"), Title: new("Look up charges"),
			Description: new("List recent subscription charges for the signed-in customer and count duplicates."),
			SideEffect:  new("read"), Method: new("GET"), PathTemplate: new("/customers/{contact.external_id}/charges"),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"month"},
				"properties": map[string]any{"month": map[string]any{"type": "string"}},
			}),
			ResponseFields: fields(
				"duplicate_count", "duplicate_count", "amount", "amount", "days_since_charge", "days_since_charge",
			),
			Channels: channels, Agents: &[]string{"dimmi", "operator"},
		},
		{
			Membership: owner, ConnectionID: &connID, Name: new("refund_charge"), Title: new("Refund a charge"),
			Description: new("Refund a confirmed duplicate charge back to the customer's card."),
			SideEffect:  new("write"), Approval: new("teammate"), Method: new("POST"),
			PathTemplate: new("/customers/{contact.external_id}/refunds"),
			BodyTemplate: schema(map[string]any{"amount": "{input.amount}", "reason": "duplicate"}),
			InputSchema: schema(map[string]any{
				"type": "object", "required": []string{"amount"},
				"properties": map[string]any{"amount": map[string]any{"type": "number", "minimum": 1, "maximum": 500}},
			}),
			ResponseFields: fields("refund_id", "id", "status", "status"),
			Channels:       channels, Agents: &[]string{"dimmi"},
		},
	}
}

/*
procedure saves two published versions of Refund a duplicate charge, using
@ chips and a code-checked Rule branch, plus three passing simulations.
*/
func (c lumenSeeder) procedure(
	ctx context.Context, owner authz.Membership, actions map[string]string, teamIDs map[string]ids.UUID,
) error {
	a := c.App
	v1, v2 := marketingProcedureVersions(actions, teamIDs)

	made, err := a.Actions.Procedures.Create(ctx, owner, "Refund a duplicate charge")
	if err != nil {
		return fmt.Errorf("create procedure: %w", err)
	}

	rawV1, _ := json.Marshal(v1)
	savedV1, err := a.Actions.Procedures.Save(ctx, procedures.SaveInput{
		Membership: owner, ID: made.ID, LockVersion: made.LockVersion, Definition: rawV1,
	})
	if err != nil {
		return fmt.Errorf("save procedure v1: %w", err)
	}
	pubV1, err := a.Actions.Procedures.PublishNote(ctx, owner, savedV1.ID, "Initial duplicate charge playbook.")
	if err != nil {
		return fmt.Errorf("publish procedure v1: %w", err)
	}

	rawV2, _ := json.Marshal(v2)
	savedV2, err := a.Actions.Procedures.Save(ctx, procedures.SaveInput{
		Membership: owner, ID: made.ID, LockVersion: pubV1.LockVersion, Definition: rawV2,
	})
	if err != nil {
		return fmt.Errorf("save procedure v2: %w", err)
	}
	if _, err := a.Actions.Procedures.PublishNote(
		ctx, owner, savedV2.ID, "Enforced the $500 limit in a code rule and added the Billing handover path.",
	); err != nil {
		return fmt.Errorf("publish procedure v2: %w", err)
	}

	return c.seedProcedureSimulations(ctx, owner, made.ID, actions)
}

/*
marketingProcedureVersions builds version 1 and version 2 of Refund a
duplicate charge so History shows a real step-by-step comparison.
*/
func marketingProcedureVersions(
	actions map[string]string, teamIDs map[string]ids.UUID,
) (procedure.Definition, procedure.Definition) {
	chips := procedureChips{
		lookup:  "[[action:" + actions["list_charges"] + "]]",
		refund:  "[[action:" + actions["refund_charge"] + "]]",
		billing: "[[handoff:" + models.PublicID(models.PrefixTeam, teamIDs["Billing"]) + "]]",
	}
	handover := []string{
		"The charges are more than 90 days old.",
		"The customer disputes the charge with their bank.",
	}
	desc := "The customer says they were charged twice for the same period and asks for one charge back."

	return marketingProcedureV1(desc, handover, chips), marketingProcedureV2(desc, handover, chips)
}

func marketingProcedureV1(desc string, handover []string, chips procedureChips) procedure.Definition {
	return procedure.Definition{
		Description: desc,
		Steps: []procedure.Step{
			{
				ID: "lookup", Kind: procedure.KindInstruction,
				Text: "Ask which month they were charged for and check it with " + chips.lookup + ".",
			},
			{
				ID: "refund", Kind: procedure.KindInstruction,
				Text: "If two charges match, refund the duplicate with " + chips.refund + ".",
			},
		},
		Handover: handover,
	}
}

type procedureChips struct {
	lookup, refund, billing string
}

func marketingProcedureV2(desc string, handover []string, chips procedureChips) procedure.Definition {
	return procedure.Definition{
		Description: desc,
		Steps: []procedure.Step{
			{
				ID: "lookup", Kind: procedure.KindInstruction,
				Text: "Ask which month they were charged for, then check recent invoices with " + chips.lookup + ".",
			},
			{
				ID: "eligible", Kind: procedure.KindBranch,
				Text: "Duplicate confirmed and within the $500 limit",
				Condition: &procedure.ConditionExpr{
					Mode: "all",
					Rules: []procedure.ConditionRule{
						{Field: "action.list_charges.duplicate_count", Operator: procedure.OpGte, Value: 1},
						{Field: "action.list_charges.amount", Operator: procedure.OpLte, Value: 500},
					},
				},
				ThenSteps: []procedure.Step{{
					ID: "refund", Kind: procedure.KindInstruction,
					Text: "Refund the duplicate with " + chips.refund +
						" and explain it appears on their card in 5 to 10 days.",
				}},
				ElseSteps: []procedure.Step{{
					ID: "explain", Kind: procedure.KindInstruction,
					Text: "If there is only one charge, explain what it covers. " +
						"If the refund is over $500, hand over to " + chips.billing + ".",
				}},
			},
		},
		Handover: handover,
	}
}

type seededSim struct {
	name       string
	persona    map[string]any
	messages   []map[string]any
	assertions []map[string]any
	result     map[string]any
}

/*
seedProcedureSimulations dates the published versions cleanly and writes
three passing saved simulations with realistic turns and judged checks.
*/
func (c lumenSeeder) seedProcedureSimulations(
	ctx context.Context, owner authz.Membership, procID ids.UUID, actions map[string]string,
) error {
	now := c.App.Clock.Now()
	ran := now.Add(-2 * time.Hour)
	marshal := func(v any) []byte {
		b, _ := json.Marshal(v)
		return b
	}

	return c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for _, v := range []struct {
			num int
			ago time.Duration
		}{{1, 14 * 24 * time.Hour}, {2, 2 * 24 * time.Hour}} {
			if _, err := tx.Exec(ctx, `UPDATE procedure_versions SET published_at = $3
				WHERE workspace_id = $1 AND procedure_id = $2 AND version = $4`,
				owner.Workspace.ID, procID, now.Add(-v.ago), v.num); err != nil {
				return fmt.Errorf("date v%d: %w", v.num, err)
			}
		}
		for i, sim := range marketingSimulations(actions, ran) {
			createdAt := now.Add(-time.Duration(3-i) * 24 * time.Hour)
			if _, err := tx.Exec(ctx, `INSERT INTO procedure_test_cases
				(workspace_id, procedure_id, name, persona, messages, assertions, last_result,
				 created_by_member_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				owner.Workspace.ID, procID, sim.name, marshal(sim.persona), marshal(sim.messages),
				marshal(sim.assertions), marshal(sim.result), owner.Member.ID, createdAt, ran); err != nil {
				return fmt.Errorf("insert simulation %q: %w", sim.name, err)
			}
		}
		return nil
	})
}

/*
marketingSimulations returns three passing saved simulations for Refund a
duplicate charge.
*/
func marketingSimulations(actions map[string]string, ran time.Time) []seededSim {
	return []seededSim{
		marketingSimDuplicate(actions, ran),
		marketingSimOverLimit(actions, ran),
		marketingSimDispute(ran),
	}
}

func marketingSimDuplicate(actions map[string]string, ran time.Time) seededSim {
	return seededSim{
		name: "Duplicate $49 charge in March",
		persona: map[string]any{
			"name": "Omar Haddad", "email": "omar@northwind.test", "external_id": "cus_4910",
			"channel": "widget", "verified": true,
		},
		messages: []map[string]any{{"text": "We were charged twice for March. Can you refund one?"}},
		assertions: []map[string]any{
			{"type": "procedure_matched"},
			{"type": "action_called", "action": "list_charges"},
			{"type": "action_called", "action": "refund_charge"},
			{"type": "branch_taken", "step_id": "eligible", "branch": "then"},
			{"type": "llm_rubric", "rubric": "explains the 5 to 10 day card timeline"},
		},
		result: map[string]any{
			"passed": true, "ran_at": ran,
			"turns": []map[string]any{{
				"message": "We were charged twice for March. Can you refund one?",
				"reply": "I found two $49.00 charges from March 1 and asked our Billing team to approve " +
					"the refund. Once approved, it shows on your card in 5 to 10 days.",
				"outcome": "answered",
				"calls": []map[string]any{
					{"action_id": actions["list_charges"], "tool": "list_charges", "succeeded": true},
					{"action_id": actions["refund_charge"], "tool": "refund_charge", "succeeded": true},
				},
			}},
			"assertion_results": []map[string]any{
				{"type": "procedure_matched", "passed": true, "message": "dimmi followed this procedure."},
				{"type": "action_called", "passed": true, "message": "Called Look up charges."},
				{"type": "action_called", "passed": true, "message": "Called Refund a charge."},
				{"type": "branch_taken", "passed": true, "message": "Took Then on Step 2."},
				{"type": "llm_rubric", "passed": true, "message": "Met (94% likely)"},
			},
		},
	}
}

func marketingSimOverLimit(actions map[string]string, ran time.Time) seededSim {
	return seededSim{
		name: "Refund over $500 hands over to Billing",
		persona: map[string]any{
			"name": "Elena Novak", "email": "elena@kestrel.test", "external_id": "cus_8821",
			"channel": "widget", "verified": true,
		},
		messages: []map[string]any{{"text": "Our annual $1,200 invoice billed twice today."}},
		assertions: []map[string]any{
			{"type": "procedure_matched"},
			{"type": "action_called", "action": "list_charges"},
			{"type": "action_not_called", "action": "refund_charge"},
			{"type": "handed_off", "handed_off": true},
		},
		result: map[string]any{
			"passed": true, "ran_at": ran,
			"turns": []map[string]any{{
				"message": "Our annual $1,200 invoice billed twice today.",
				"reply": "I see both $1,200 charges on your account. Because this refund is over $500, " +
					"I passed the thread to our Billing team so they can issue it right away.",
				"outcome": "handoff",
				"calls": []map[string]any{
					{"action_id": actions["list_charges"], "tool": "list_charges", "succeeded": true},
				},
			}},
			"assertion_results": []map[string]any{
				{"type": "procedure_matched", "passed": true, "message": "dimmi followed this procedure."},
				{"type": "action_called", "passed": true, "message": "Called Look up charges."},
				{"type": "action_not_called", "passed": true, "message": "Did not call Refund a charge."},
				{"type": "handed_off", "passed": true, "message": "Handed over to Billing."},
			},
		},
	}
}

func marketingSimDispute(ran time.Time) seededSim {
	return seededSim{
		name: "Bank dispute triggers immediate handover",
		persona: map[string]any{
			"name": "Dev Patel", "email": "dev@beacon.test", "external_id": "cus_2014",
			"channel": "email", "verified": true,
		},
		messages: []map[string]any{
			{"text": "I already opened a chargeback with my bank for this duplicate charge."},
		},
		assertions: []map[string]any{
			{"type": "procedure_matched"},
			{"type": "handed_off", "handed_off": true},
		},
		result: map[string]any{
			"passed": true, "ran_at": ran,
			"turns": []map[string]any{{
				"message": "I already opened a chargeback with my bank for this duplicate charge.",
				"reply": "I have passed this straight to our Billing team so they can handle " +
					"the bank dispute with you.",
				"outcome": "handoff",
				"calls":   []map[string]any{},
			}},
			"assertion_results": []map[string]any{
				{"type": "procedure_matched", "passed": true, "message": "dimmi followed this procedure."},
				{"type": "handed_off", "passed": true, "message": "Handed over to Billing."},
			},
		},
	}
}
