/*
 * The edits Operator suggested to the procedure open in the editor,
 * above its steps, each with what it changes step by step. Applying one
 * writes the working copy, so the editor takes it in; nothing goes live
 * until a teammate publishes.
 */
import { RiSparkling2Line } from "@remixicon/react";

import { ChangesetCard } from "@/features/operator/changeset-card";
import { ProcedureChanges } from "@/features/operator/suggested-edits";
import { useSuggestions } from "@/features/operator/suggestions";

export function ProcedureSuggestions({
  workspace,
  procedure,
  onApplied,
}: {
  workspace: string;
  procedure: string;
  onApplied: () => void;
}) {
  const { suggestions, decide } = useSuggestions(
    workspace,
    (op) =>
      (op.kind === "procedure.edit_draft" || op.kind === "procedure.create_test_case") && op.procedure_id === procedure,
    (_, decision) => {
      if (decision !== "declines") {
        onApplied();
      }
    },
  );
  if (suggestions.length === 0) {
    return null;
  }
  return (
    <section aria-label="Suggested edits" className="mx-auto flex w-full max-w-220 flex-col gap-2 px-4 pt-6 sm:px-6">
      <h2 className="flex items-center gap-2 text-body-2-medium text-text-secondary">
        <RiSparkling2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        {suggestions.length === 1 ? "Operator suggested an edit" : `Operator suggested ${suggestions.length} edits`}
      </h2>
      {suggestions.map((c) => (
        <ChangesetCard key={c.id} changeset={c} onDecide={decide}>
          <ProcedureChanges workspace={workspace} changeset={c} />
        </ChangesetCard>
      ))}
    </section>
  );
}
