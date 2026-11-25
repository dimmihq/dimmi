/*
|--------------------------------------------------------------------------
| Tool Catalog: Automation
|--------------------------------------------------------------------------
|
| The entries for the AI agent, its actions and keys, Operator, procedures and workflows.
|
*/

package mcpserver

import "maps"

/*
automationDocs returns the entries for the AI agent, its actions and
keys, Operator, procedures and workflows.
*/
func automationDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		agentDocs(), keyDocs(), aiActionDocs(), operatorProcedureDocs(), agentStudioDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
agentDocs returns the entry for what the AI agents cost. The agents
themselves are in catalog_agent_studio.go.
*/
func agentDocs() map[string]Doc {
	return map[string]Doc{
		"ai.usage.show": {
			Title: "Get AI agent usage",
			Use: "Reads one month of the AI agent's work: what it spent, tokens, how many conversations it " +
				"answered, did not know or handed over, day by day and by model. Months are counted in UTC. " +
				"Operator's own spend is in dimmi_get_operator_usage.",
			Returns: "the month's cost in micro-dollars, counts by outcome, daily series, spend by model and the cap.",
			Params: map[string]string{
				"period": "The month as YYYY-MM, such as 2026-11. Leave out for the current month.",
			},
		},
	}
}

/*
keyDocs returns the entries for the AI agent's provider keys.
*/
func keyDocs() map[string]Doc {
	return map[string]Doc{
		"ai.keys.index": {
			Title:   "List AI provider keys",
			Use:     "Lists the AI provider connections the agents answer with. Secrets never come back, only a hint.",
			Returns: "every key with its provider, status, the models it offers and when it was last checked.",
		},
		"ai.keys.store": {
			Title: "Add an AI provider key",
			Use: "Connects an AI provider with the workspace's own API key. dimmi checks the key with the " +
				"provider as it saves; a key the provider refuses is not kept. Only add a key the person gave " +
				"you for this.",
			Returns: "the key with its status and the models it offers, the secret hidden.",
			Next: []string{
				"dimmi_update_ai_settings: let every agent answer with the new key",
				"dimmi_update_ai_agent: point one agent at the new key with answer_key_id",
			},
			Params: map[string]string{
				"provider": "anthropic, openai, engy or openai_compatible. ChatGPT plans connect in the browser " +
					"instead.",
				"api_key":  "The provider's secret key. Required except for an openai_compatible server without one.",
				"base_url": "Endpoint of an openai_compatible server, such as https://llm.example.com/v1.",
			},
			Hint: "A workspace holds at most 20 keys; remove an unused one first.",
		},
		"ai.keys.update": {
			Title: "Change an AI provider key",
			Use: "Renames a provider key, moves its endpoint or rotates its secret. A new secret or endpoint " +
				"is checked again at once, and the agents switch to it on their next answer.",
			Returns: "the key with its fresh status.",
			Params:  map[string]string{"api_key": "A new secret to replace the stored one; leave out to keep it."},
		},
		"ai.keys.destroy": {
			Title: "Remove an AI provider key",
			Use: "Deletes a provider key and its secret. Refused while an agent or the shared settings still " +
				"answer with it: point them at another key with dimmi_update_ai_agent or dimmi_update_ai_settings " +
				"first.",
		},
		"ai.keys.test.store": {
			Title:   "Check an AI provider key",
			Use:     "Checks a provider key with the provider again and stores the fresh status.",
			Returns: "the key with its status, a reason when it fails, and the models it offers.",
			Risk:    new(RiskWrite),
		},
	}
}
