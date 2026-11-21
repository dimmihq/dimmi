/*
 * What the procedure simulator and saved tests share: the assertions a
 * test can make, reading typed JSON, and the assertions a chat saved as
 * a test starts with.
 */
import type { Assertion, Simulation } from "@/lib/api/procedures";

import type { Choices } from "../choices";

export const assertionChoices: Choices = [
  ["procedure_matched", "The procedure matched"],
  ["action_called", "An action was called"],
  ["action_not_called", "An action was not called"],
  ["action_order", "One action ran before another"],
  ["branch_taken", "A rule took a side"],
  ["handed_off", "It was handed over"],
  ["reply_contains", "A reply says"],
  ["reply_not_contains", "No reply says"],
  ["llm_rubric", "A rubric, judged by a model"],
];

/*
 * readJSON reads what was typed as JSON, or keeps it as text.
 */
export function readJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

/*
 * defaultAssertions are what a chat saved as a test checks at first:
 * the procedure matched, and every action it called was called again.
 */
export function defaultAssertions(turns: Simulation[]): Assertion[] {
  const out: Assertion[] = [{ type: "procedure_matched" }];
  const seen = new Set<string>();
  for (const turn of turns) {
    for (const call of turn.action_calls) {
      if (!call.blocked && !seen.has(call.tool)) {
        seen.add(call.tool);
        out.push({ type: "action_called", action: call.tool });
      }
    }
  }
  return out;
}
