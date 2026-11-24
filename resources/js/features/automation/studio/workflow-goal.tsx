/*
 * A workflow's goal, set beside its trigger: what a run that worked
 * looks like, so Analytics can count it. The conversation is resolved
 * without going to a person, or the customer rates it at least a
 * score, within a window after the run starts. A goal never changes
 * what the workflow does.
 */
import { Row, Section } from "@/features/settings/layout";
import type { Spec, WorkflowGoal } from "@/lib/api/workflows";

import type { Choices } from "../choices";
import { Pick } from "../pick";

const none = "none";

const kindChoices: Choices = [
  [none, "No goal"],
  ["resolved", "Resolved without a handover"],
  ["csat", "Rated at least"],
];

const windowChoices: Choices = [
  ["1d", "1 day"],
  ["3d", "3 days"],
  ["7d", "7 days"],
  ["14d", "14 days"],
  ["30d", "30 days"],
];

const ratingChoices: Choices = [
  ["3", "3 of 5"],
  ["4", "4 of 5"],
  ["5", "5 of 5"],
];

export function WorkflowGoalSettings({ spec, onSpec }: { spec: Spec; onSpec: (edit: (s: Spec) => Spec) => void }) {
  const goal = spec.goal ?? null;
  const set = (next: WorkflowGoal | null) => onSpec((current) => ({ ...current, goal: next }));

  return (
    <Section title="Goal" description="What a run that worked looks like. Analytics counts it; runs never change.">
      <Row label="Goal">
        <Pick
          label="Goal"
          choices={kindChoices}
          value={goal?.kind ?? none}
          onChange={(kind) =>
            set(
              kind === none
                ? null
                : {
                    kind: kind as WorkflowGoal["kind"],
                    window: goal?.window ?? "7d",
                    min_rating: kind === "csat" ? (goal?.min_rating ?? 4) : undefined,
                  },
            )
          }
        />
      </Row>
      {goal?.kind === "csat" ? (
        <Row label="Lowest rating">
          <Pick
            label="Lowest rating"
            choices={ratingChoices}
            value={String(goal.min_rating ?? 4)}
            onChange={(rating) => set({ ...goal, min_rating: Number(rating) })}
          />
        </Row>
      ) : null}
      {goal ? (
        <Row label="Within" description="Counted from when the run starts.">
          <Pick
            label="Goal window"
            choices={windowChoices}
            value={goal.window ?? "7d"}
            onChange={(window) => set({ ...goal, window })}
          />
        </Row>
      ) : null}
    </Section>
  );
}
