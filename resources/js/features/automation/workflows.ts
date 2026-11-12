/*
 * Workflows as the builder edits them: the server's definition, a tree
 * of steps where steps with paths (reply buttons, branches) continue
 * down whichever path applies. Paths people add carry a name; fixed
 * paths carry the outcome that takes them and read under its name.
 * Steps and paths get their ids here, in the shape the server accepts,
 * so a step keeps its id from the first edit on.
 */
import type { SpecPath, SpecStep } from "@/lib/api/workflows";

export type StepKind =
  | "message"
  | "buttons"
  | "collect"
  | "reply_time"
  | "dimmi"
  | "branch"
  | "classify"
  | "wait"
  | "run_workflow"
  | "assign"
  | "unassign"
  | "tag"
  | "untag"
  | "priority"
  | "note"
  | "update_contact"
  | "snooze"
  | "close"
  | "apply_sla"
  | "http_request"
  | "create_ticket"
  | "set_ticket_state"
  | "set_ticket_fields"
  | "escalate"
  | "add_to_tracker";

export type Step = SpecStep;
export type Path = SpecPath;

/*
 * readyKinds are the step kinds the server runs today, the only ones the
 * builder offers; the others still show when a definition holds them.
 */
export const readyKinds: StepKind[] = [
  "message",
  "buttons",
  "collect",
  "reply_time",
  "dimmi",
  "branch",
  "classify",
  "wait",
  "run_workflow",
  "assign",
  "unassign",
  "tag",
  "untag",
  "priority",
  "note",
  "update_contact",
  "snooze",
  "close",
  "http_request",
  "apply_sla",
  "create_ticket",
  "set_ticket_state",
  "set_ticket_fields",
  "escalate",
  "add_to_tracker",
];

/*
 * customerKinds speak to the customer, which only some triggers may do.
 */
export const customerKinds: StepKind[] = ["message", "buttons", "collect", "reply_time"];

const outcomes: Record<string, string> = {
  typed: "Typed an answer",
  timeout: "No answer in time",
  invalid: "Invalid answer",
  resolved: "Resolved",
  handed_over: "Handed over",
  quiet: "Customer went quiet",
  else: "Everyone else",
  none: "None matched or AI unavailable",
  interrupted: "Cut short",
  failure: "Failed",
};

/*
 * pathName is what a path reads as: its own name, or its outcome's.
 */
export function pathName(path: Path): string {
  if (path.outcome) {
    return outcomes[path.outcome] ?? path.outcome;
  }

  return path.label ?? "";
}

/*
 * newId makes a step or path id the server accepts: its prefix and ten
 * random lowercase letters and digits.
 */
export function newId(prefix: "s_" | "p_"): string {
  const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789";
  const bytes = crypto.getRandomValues(new Uint8Array(10));

  return prefix + Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join("");
}

/*
 * triggerStep names the trigger card when it is the selected thing in the
 * builder, next to the steps' own ids.
 */
export const triggerStep = "trigger";

/*
 * findStep returns the step with an id anywhere in a list of steps,
 * including down every path.
 */
export function findStep(steps: Step[], id: string): Step | undefined {
  for (const step of steps) {
    if (step.id === id) {
      return step;
    }
    for (const path of step.paths ?? []) {
      const inner = findStep(path.steps, id);
      if (inner) {
        return inner;
      }
    }
  }

  return undefined;
}
