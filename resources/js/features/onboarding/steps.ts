/*
 * The setup steps of a new workspace, in the order its owners walk them.
 * The server keeps the step they reached, so setup resumes where it was
 * left, on any device.
 */
export const steps = ["workspace", "logo", "channels", "tags", "team", "knowledge", "test_conversations"] as const;

export type Step = (typeof steps)[number] | "done";

export function stepFrom(value: string | undefined): Step {
  return value === "done" || steps.includes(value as (typeof steps)[number]) ? (value as Step) : "workspace";
}

/*
 * nextStep is the step after the given one; after the last comes the
 * summary.
 */
export function nextStep(step: Step): Step {
  const index = steps.indexOf(step as (typeof steps)[number]);

  return index < 0 || index === steps.length - 1 ? "done" : steps[index + 1];
}

export function previousStep(step: Step): Step {
  const index = steps.indexOf(step as (typeof steps)[number]);

  return index <= 0 ? "workspace" : steps[index - 1];
}

/*
 * position is the step's number, counting from one; the summary counts
 * as every step done.
 */
export function position(step: Step): number {
  return step === "done" ? steps.length : steps.indexOf(step) + 1;
}
