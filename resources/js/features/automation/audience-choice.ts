/*
 * Who a workflow runs for, as the builder offers it: everyone, or the
 * people in one saved audience. A workflow whose audience was written
 * another way, through the API, keeps it and reads as a custom rule
 * until someone picks again.
 */
import type { AudienceRules } from "@/lib/api/audiences";

export const everyone = "everyone";
export const custom = "custom";

/*
 * audienceChoice is the builder's choice for a stored audience: everyone,
 * a saved audience's id, or custom.
 */
export function audienceChoice(audience: AudienceRules | undefined): string {
  if (!audience || ((audience.visitors ?? "everyone") === "everyone" && !audience.rules)) {
    return everyone;
  }
  const rule = audience.rules;
  const lone =
    (audience.visitors ?? "everyone") === "everyone" &&
    rule &&
    "field" in rule &&
    rule.field === "audience" &&
    rule.op === "in" &&
    Array.isArray(rule.value) &&
    rule.value.length === 1;

  return lone ? String((rule.value as unknown[])[0]) : custom;
}

/*
 * audienceFor is the stored audience of a choice.
 */
export function audienceFor(choice: string): AudienceRules | undefined {
  if (choice === everyone || choice === custom) {
    return undefined;
  }

  return { rules: { field: "audience", op: "in", value: [choice] } };
}
