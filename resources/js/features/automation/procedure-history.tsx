/*
 * A procedure's published versions, newest first: who published each
 * and why, and how many conversations followed it and went to the team.
 * Any two versions, or a version and the working copy, compare step by
 * step. A version can be put back as the working copy, to look over
 * before publishing, or published at once.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { Row, Section } from "@/features/settings/layout";
import {
  type ProcedureDefinition,
  type ProcedureStep,
  type ProcedureVersion,
  restoreVersion,
  versionsQuery,
} from "@/lib/api/procedures";
import { countOf } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import type { Choices } from "./choices";
import { Pick } from "./pick";
import { diffSteps, type StepChange } from "./procedures";
import type { useProcedureDraft } from "./use-procedure-draft";

type Draft = ReturnType<typeof useProcedureDraft>;

const working = "working";

const changeWords: Record<StepChange["change"], string> = {
  added: "Added",
  removed: "Removed",
  changed: "Changed",
  same: "Same",
};

function stepLine(step?: ProcedureStep): string {
  if (!step) {
    return "";
  }
  const rule = step.condition
    ? ` [rule: ${step.condition.rules.map((r) => `${r.field} ${r.operator} ${JSON.stringify(r.value ?? "")}`).join(` ${step.condition.mode} `)}]`
    : "";
  return `${step.kind}: ${step.text}${rule}`;
}

function Diff({ before, after }: { before: ProcedureDefinition; after: ProcedureDefinition }) {
  const changes = diffSteps(before, after).filter((c) => c.change !== "same");
  const described = before.description !== after.description;

  if (changes.length === 0 && !described) {
    return <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">The steps are the same.</p>;
  }

  return (
    <div className="flex flex-col">
      {described ? (
        <Row label="When dimmi uses this" description={`${before.description} → ${after.description}`} />
      ) : null}
      {changes.map((c) => (
        <div key={c.id} className="flex flex-col gap-1 border-b border-separator-border px-3 py-2.5 last:border-b-0">
          <span className="flex items-center gap-2">
            <span className="text-body-2-medium text-text-tertiary tabular-nums">{c.label}</span>
            <Badge color={c.change === "removed" ? "primary" : "neutral"}>{changeWords[c.change]}</Badge>
          </span>
          {c.before ? (
            <span
              className={cx("text-body-2-regular", c.after ? "text-text-tertiary line-through" : "text-text-secondary")}
            >
              {stepLine(c.before)}
            </span>
          ) : null}
          {c.after ? <span className="text-body-2-regular text-text-primary">{stepLine(c.after)}</span> : null}
        </div>
      ))}
    </div>
  );
}

export function ProcedureHistory({
  workspace,
  draft,
  readOnly,
}: {
  workspace: string;
  draft: Draft;
  readOnly: boolean;
}) {
  const id = draft.server.id;
  const versions = useQuery(versionsQuery(workspace, id));
  const list = versions.data ?? [];
  const [left, setLeft] = useState<string>("");
  const [right, setRight] = useState<string>(working);
  const [busy, setBusy] = useState<number | null>(null);
  const choices: Choices = [
    [working, "Working copy"],
    ...list.map((v) => [String(v.version), `Version ${v.version}`] as const),
  ];
  const pick = (key: string): ProcedureDefinition | undefined =>
    key === working ? draft.definition : list.find((v) => String(v.version) === key)?.definition;
  const leftKey = left || (list[0] ? String(list[0].version) : "");
  const before = pick(leftKey);
  const after = pick(right);

  const restore = async (v: ProcedureVersion, publish: boolean) => {
    setBusy(v.version);
    try {
      await draft.flush();
      const next = await restoreVersion(workspace, id, v.version, { publish, lock_version: draft.server.lock_version });
      draft.adopt(next);
      void versions.refetch();
      toast.success(publish ? `Version ${v.version} is live again.` : `Version ${v.version} is the working copy.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The version could not be restored.");
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-8 pb-16">
      <Section title="Compare" description="Pick two versions, or a version and the working copy.">
        <Row label="From and to">
          <Pick key={`l${leftKey}`} label="Compare from" choices={choices} value={leftKey} onChange={setLeft} />
          <Pick key={`r${right}`} label="Compare to" choices={choices} value={right} onChange={setRight} />
        </Row>
        {before && after ? <Diff before={before} after={after} /> : null}
      </Section>
      <Section title="Versions" loading={versions.isPending}>
        {list.length === 0 ? (
          <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">Publish to keep the first version.</p>
        ) : null}
        {list.map((v) => (
          <Row
            key={v.id}
            label={`Version ${v.version}${draft.server.status === "live" && v === list[0] ? " · live" : ""}`}
            description={`${new Date(v.published_at).toLocaleString()} · ${v.change_note ?? "No note"} · ${countOf(
              v.conversations,
              "conversation",
              "conversations",
            )}, ${v.handed_off} handed over`}
          >
            {readOnly ? null : (
              <>
                <Button
                  variant="secondary"
                  size="small"
                  disabled={busy !== null}
                  onClick={() => void restore(v, false)}
                >
                  Restore to draft
                </Button>
                <Button
                  size="small"
                  loading={busy === v.version}
                  disabled={busy !== null}
                  onClick={() => void restore(v, true)}
                >
                  Publish this
                </Button>
              </>
            )}
          </Row>
        ))}
      </Section>
    </div>
  );
}
