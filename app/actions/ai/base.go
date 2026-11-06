/*
|--------------------------------------------------------------------------
| AI Action Helpers
|--------------------------------------------------------------------------
|
| The small shared bits the agent's actions lean on: the permissions
| that gate them and the model catalog answers. Every member may read;
| changing the agent, its connections, and reading the prompts
| themselves are for the people who run the workspace.
|
*/

package ai

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
RequireAgentRead checks the caller may see the agent's settings, its
connections without their secrets, its runs and its usage.
*/
func RequireAgentRead(m authz.Membership) error {
	if !m.Can("ai:read") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
RequireAgentManage checks the caller may change the agent: its
settings, whether it answers at all and what it may spend.
*/
func RequireAgentManage(m authz.Membership) error {
	if !m.Can("ai_agent:manage") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
RequireAgentKeys checks the caller may manage provider connections,
which spend the workspace's money.
*/
func RequireAgentKeys(m authz.Membership) error {
	if !m.Can("ai:keys") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
RequirePlayground checks the caller may ask the agent a test question.
*/
func RequirePlayground(m authz.Membership) error {
	if !m.Can("ai:write") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
CanReadPayloads reports whether the caller may read the redacted
prompts and replies a run's steps carry; everyone else sees only the
numbers.
*/
func CanReadPayloads(m authz.Membership) bool {
	return m.Can("ai_runs:read_payloads")
}

/*
ProviderDescriptor describes one connectable provider for the picker.
*/
type ProviderDescriptor struct {
	ID          enums.AIProvider `json:"id"`
	Name        string           `json:"name"`
	AuthType    string           `json:"auth_type"`
	KeyRequired bool             `json:"key_required"`
	DocsURL     string           `json:"docs_url"`
	Available   bool             `json:"available"`
}

/*
CatalogEntry is one selectable model for the settings UI.
*/
type CatalogEntry struct {
	ID                 string              `json:"id"`
	DisplayName        string              `json:"display_name"`
	Family             enums.AIModelFamily `json:"family"`
	Suggested          []enums.AIModelRole `json:"suggested"`
	ContextTokens      int                 `json:"context_tokens"`
	InputPerMtok       *string             `json:"input_per_mtok"`
	OutputPerMtok      *string             `json:"output_per_mtok"`
	CachedInputPerMtok *string             `json:"cached_input_per_mtok"`
}

/*
findKey returns one live key of the workspace or 404.
*/
func findKey(ctx context.Context, tx *database.Tx, workspaceID, keyID ids.UUID) (models.AiProviderKey, error) {
	key, err := models.New(tx).FindProviderKey(ctx, models.FindProviderKeyParams{
		WorkspaceID: workspaceID, ID: keyID,
	})
	if err != nil {
		return models.AiProviderKey{}, fmt.Errorf("find provider key: %w", exceptions.ErrNotFound)
	}
	return key, nil
}

/*
keyInUse reports whether an agent points a role at the key.
*/
func keyInUse(agent models.AiAgent, keyID ids.UUID) []string {
	var roles []string
	if agent.AnswerKeyID != nil && *agent.AnswerKeyID == keyID {
		roles = append(roles, "answer")
	}
	if agent.FallbackKeyID != nil && *agent.FallbackKeyID == keyID {
		roles = append(roles, "fallback")
	}
	return roles
}

/*
catalogEntry maps a catalog row for the API.
*/
func catalogEntry(e aimodels.Entry, prices *aiprices.Table) CatalogEntry {
	out := CatalogEntry{
		ID: e.ID, DisplayName: e.DisplayName, Family: e.Family,
		Suggested: e.Suggested, ContextTokens: e.ContextTokens,
	}
	if prices != nil {
		if p := prices.Find(string(enums.AIProviderDimmi), e.ID); p != nil {
			out.InputPerMtok = new(p.InputPerMtok)
			out.OutputPerMtok = new(p.OutputPerMtok)
			out.CachedInputPerMtok = new(p.CachedInputPerMtok)
		}
	}
	return out
}
