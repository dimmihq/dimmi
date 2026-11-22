/*
 * Suggested edits shown where the content lives: the help center lists
 * the article drafts and edits Operator suggested, an article shows the
 * edits suggested to it with what each changes line by line, and the
 * procedures do the same for their working copies. Each is the same card
 * as in Operator's threads, to apply or decline right here. Dry run
 * previews never show up here.
 */
import { RiSparkling2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Diff } from "@/features/automation/procedure-history";
import { useChipSources } from "@/features/automation/procedure-writer/use-chip-sources";
import { articleQuery } from "@/lib/api/help-centers";
import type { OperatorChangeset, OperatorOperation } from "@/lib/api/operator";
import { type ProcedureDefinition, procedureQuery } from "@/lib/api/procedures";
import { cx } from "@/utils/cx";

import { ChangesetCard } from "./changeset-card";
import { changeCount, diffLines, folded } from "./line-diff";
import { useSuggestions } from "./suggestions";

/*
 * SuggestedEdits is a section of suggestion cards, nothing when none
 * waits.
 */
export function SuggestedEdits({
  workspace,
  match,
  description,
  render,
}: {
  workspace: string;
  match: (op: OperatorOperation) => boolean;
  description: string;
  render?: (changeset: OperatorChangeset) => ReactNode;
}) {
  const { suggestions, decide } = useSuggestions(workspace, match);
  if (suggestions.length === 0) {
    return null;
  }
  return (
    <section aria-label="Suggested edits" className="flex flex-col gap-2">
      <div className="flex flex-col gap-0.5 px-3">
        <h2 className="flex items-center gap-2 text-body-2-medium text-text-secondary">
          <RiSparkling2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
          Suggested edits
          <Chip variant="caption" color="blue" className="py-0">
            {suggestions.length}
          </Chip>
        </h2>
        <p className="text-body-2-regular text-text-tertiary">{description}</p>
      </div>
      <div className="flex flex-col gap-2">
        {suggestions.map((c) => (
          <ChangesetCard key={c.id} changeset={c} onDecide={decide}>
            {render?.(c)}
          </ChangesetCard>
        ))}
      </div>
    </section>
  );
}

/*
 * ArticleChanges shows what each article edit of a changeset changes in
 * the article's working copy, line by line.
 */
export function ArticleChanges({ workspace, changeset }: { workspace: string; changeset: OperatorChangeset }) {
  return (
    <>
      {changeset.operations
        .filter((op) => op.kind === "article.edit_draft" && op.article_id && op.text)
        .map((op) => (
          <ArticleDiff
            key={op.article_id}
            workspace={workspace}
            article={op.article_id ?? ""}
            proposed={op.text ?? ""}
          />
        ))}
    </>
  );
}

function ArticleDiff({ workspace, article, proposed }: { workspace: string; article: string; proposed: string }) {
  const { data } = useQuery(articleQuery(workspace, article));
  if (!data) {
    return null;
  }
  const lines = diffLines(data.body_markdown ?? "", proposed);
  return <DiffView lines={folded(lines)} title={data.title} counts={changeCount(lines)} />;
}

/*
 * DiffView shows a folded line diff: removed lines red, added lines
 * green, unchanged lines plain.
 */
function DiffView({
  lines,
  title,
  counts,
}: {
  lines: ReturnType<typeof folded>;
  title: string;
  counts: { added: number; removed: number };
}) {
  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-separator-border">
      <div className="flex items-center justify-between gap-2 border-b border-separator-border bg-background-secondary-default px-2.5 py-1.5">
        <span className="truncate text-caption-1-medium text-text-secondary">{title}</span>
        <span className="shrink-0 text-caption-1-medium tabular-nums">
          <span className="text-status-lime-text">+{counts.added}</span>{" "}
          <span className="text-status-rose-text">−{counts.removed}</span>
        </span>
      </div>
      <div className="max-h-80 overflow-y-auto font-mono text-caption-1-regular">
        {lines.map((l, i) => (
          <div
            // biome-ignore lint/suspicious/noArrayIndexKey: diff lines repeat and keep their order
            key={i}
            className={cx(
              "flex gap-2 whitespace-pre-wrap px-2.5 py-0.5",
              l.kind === "add" && "bg-status-lime-background text-text-primary",
              l.kind === "del" && "bg-status-rose-background text-text-secondary line-through",
              l.kind === "same" && "text-text-secondary",
              l.kind === "fold" && "bg-background-secondary-default text-text-tertiary italic",
            )}
          >
            <span aria-hidden className="w-3 shrink-0 select-none">
              {l.kind === "add" ? "+" : l.kind === "del" ? "−" : ""}
            </span>
            <span className="min-w-0">{l.text || " "}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

/*
 * ProcedureChanges shows what a procedure edit changes in the working
 * copy, step by step.
 */
export function ProcedureChanges({ workspace, changeset }: { workspace: string; changeset: OperatorChangeset }) {
  return (
    <>
      {changeset.operations
        .filter((op) => op.kind === "procedure.edit_draft" && op.procedure_id && op.text)
        .map((op) => (
          <ProcedureDiff
            key={op.procedure_id}
            workspace={workspace}
            procedure={op.procedure_id ?? ""}
            proposed={op.text ?? ""}
          />
        ))}
    </>
  );
}

function ProcedureDiff({ workspace, procedure, proposed }: { workspace: string; procedure: string; proposed: string }) {
  const { data } = useQuery(procedureQuery(workspace, procedure));
  const sources = useChipSources(workspace, procedure);
  let after: ProcedureDefinition | null = null;
  try {
    after = { ...(data?.definition ?? {}), ...JSON.parse(proposed) } as ProcedureDefinition;
  } catch {
    after = null;
  }
  if (!data || !after) {
    return null;
  }
  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-separator-border">
      <div className="border-b border-separator-border bg-background-secondary-default px-2.5 py-1.5">
        <span className="truncate text-caption-1-medium text-text-secondary">{data.name}</span>
      </div>
      <div className="ps-3">
        <Diff before={data.definition} after={after} sources={sources} />
      </div>
    </div>
  );
}
