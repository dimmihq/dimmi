/*
 * The audience picked for each part of a messenger: the slots, the
 * choices picked but not saved yet, and what a save sends and says when
 * the server refuses it.
 */
import { audienceChoice, audienceFor } from "@/features/automation/audience-choice";
import { ApiError } from "@/lib/api/client";
import type { MessengerAudiences } from "@/lib/api/widget-settings";

export type Slot = "launcher" | "start" | "help" | "changelog";

function ruleOf(rules: MessengerAudiences, slot: Slot) {
  return (slot === "launcher" || slot === "start" ? rules[slot] : rules.tabs?.[slot]) ?? undefined;
}

/*
 * AudienceChoices are the choices picked but not saved yet, by slot.
 */
export type AudienceChoices = Partial<Record<Slot, string>>;

/*
 * savedChoice is the choice a slot holds as saved.
 */
export function savedChoice(rules: MessengerAudiences | undefined, slot: Slot): string {
  return audienceChoice(ruleOf(rules ?? {}, slot));
}

/*
 * audienceChange is what to send for the choices picked: each slot's
 * rule, null for everyone.
 */
export function audienceChange(choices: AudienceChoices): MessengerAudiences {
  const out: MessengerAudiences = {};
  for (const [slot, choice] of Object.entries(choices) as [Slot, string][]) {
    const rule = audienceFor(choice) ?? null;
    if (slot === "launcher" || slot === "start") {
      out[slot] = rule;
    } else {
      out.tabs = { ...out.tabs, [slot]: rule };
    }
  }

  return out;
}

/*
 * audienceProblem is the sentence for refused rules: the first one the
 * server names, else its message.
 */
export function audienceProblem(error: unknown): string {
  const list = error instanceof ApiError ? (error.details.errors as { message: string }[] | undefined) : undefined;

  return list?.[0]?.message ?? (error instanceof Error ? error.message : "Not saved.");
}
