/*
 * The edits Operator suggested to the article open in the editor, above
 * the article, each with what it changes line by line. Applying one
 * writes the article's working copy, so the editor reloads it; nothing
 * is published until a teammate publishes.
 */
import { RiSparkling2Line } from "@remixicon/react";

import { ChangesetCard } from "@/features/operator/changeset-card";
import { ArticleChanges } from "@/features/operator/suggested-edits";
import { useSuggestions } from "@/features/operator/suggestions";

export function ArticleSuggestions({
  workspace,
  article,
  onApplied,
}: {
  workspace: string;
  article: string;
  onApplied: () => void;
}) {
  const { suggestions, decide } = useSuggestions(
    workspace,
    (op) => op.kind === "article.edit_draft" && op.article_id === article,
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
    <section aria-label="Suggested edits" className="mx-auto flex w-full max-w-[720px] flex-col gap-2 px-6 pt-6">
      <h2 className="flex items-center gap-2 text-body-2-medium text-text-secondary">
        <RiSparkling2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        {suggestions.length === 1 ? "Operator suggested an edit" : `Operator suggested ${suggestions.length} edits`}
      </h2>
      {suggestions.map((c) => (
        <ChangesetCard key={c.id} changeset={c} onDecide={decide}>
          <ArticleChanges workspace={workspace} changeset={c} />
        </ChangesetCard>
      ))}
    </section>
  );
}
