/*
|--------------------------------------------------------------------------
| Loading a Workspace's Redaction
|--------------------------------------------------------------------------
|
| Every write reads the workspace's switches and its enabled rules inside
| its own transaction, so a rule takes effect on the very next message
| and a deleted one stops at once. A workspace that never touched its
| settings gets the defaults: card numbers on, social security numbers
| off.
|
*/

package redaction

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/redaction"
)

/*
Load returns the redactor of the workspace.
*/
func Load(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (redaction.Redactor, error) {
	settings, err := readSettings(ctx, q, workspaceID)
	if err != nil {
		return redaction.Redactor{}, err
	}

	rows, err := q.ListEnabledRedactionRules(ctx, workspaceID)
	if err != nil {
		return redaction.Redactor{}, fmt.Errorf("list redaction rules: %w", err)
	}

	return redactor(settings, rows)
}

/*
readSettings returns the workspace's built-in switches over the defaults.
*/
func readSettings(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (redaction.Settings, error) {
	settings := redaction.DefaultSettings()

	raw, err := q.GetRedactionSettings(ctx, workspaceID)
	if err != nil {
		return settings, fmt.Errorf("read redaction settings: %w", err)
	}

	if err := json.Unmarshal(raw, &settings); err != nil {
		return settings, fmt.Errorf("decode redaction settings: %w", err)
	}

	return settings, nil
}

/*
redactor compiles the rules under their public ids, which name them in
a message's hits.
*/
func redactor(settings redaction.Settings, rows []models.RedactionRule) (redaction.Redactor, error) {
	rules := make([]redaction.Rule, 0, len(rows))
	for _, row := range rows {
		rules = append(rules, redaction.Rule{
			Key:         models.PublicID(models.PrefixRedactionRule, row.ID),
			Pattern:     row.Pattern,
			Replacement: row.Replacement,
		})
	}

	r, err := redaction.New(settings, rules)
	if err != nil {
		return r, fmt.Errorf("compile redaction rules: %w", err)
	}

	return r, nil
}
