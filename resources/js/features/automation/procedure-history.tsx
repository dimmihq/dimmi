/*
 * A procedure's published versions, newest first: when each was
 * published and why, and how many conversations followed it and went to
 * the team. Any two versions, or a version and the working copy,
 * compare step by step, each change in the step's own words. From a
 * version's menu it can be compared with the working copy, put back as
 * the working copy to look over before publishing, or published at
 * once, which asks first since dimmi follows it straight away.
 */
import { RiArrowRightLine, RiMore2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Section } from "@/features/settings/layout";
import {
  type ProcedureDefinition,
  type ProcedureStep,
  type ProcedureVersion,
  restoreVersion,
  versionsQuery,
} from "@/lib/api/procedures";
import { countOf, formatDateTime, formatRelative } from "@/lib/i18n/format";

import type { Choices } from "./choices";
import { Pick } from "./pick";
import type { ChipSources } from "./procedure-writer/procedure-suggestions";
import { diffSteps, type StepChange } from "./procedures";
import { kindWords, stepTitle } from "./step-words";
import type { useProcedureDraft } from "./use-procedure-draft";

type Draft = ReturnType<typeof useProcedureDraft>;

const working = "working";

const changes: Record<Exclude<StepChange["change"], "same">, { label: string; color: ChipColor }> = {
  added: { label: "Added", color: "lime" },
  removed: { label: "Removed", color: "rose" },
  changed: { label: "Changed", color: "blue" },
};

const line = "flex flex-col gap-1.5 border-b border-separator-border py-3 pe-3 last:border-b-0";
const was = "text-body-2-regular text-text-tertiary line-through";
const now = "text-body-2-regular text-text-primary";

/*
 * ruleLine is a rule's conditions in one line, or nothing for a step
 * without a rule.
 */
function ruleLine(step?: ProcedureStep): string {
  if (!step?.condition) {
    return "";
  }
  const joiner = step.condition.mode === "all" ? " and " : " or ";

  return step.condition.rules.map((r) => `${r.field} ${r.operator} ${String(r.value ?? "")}`.trim()).join(joiner);
}

function kindOf(step?: ProcedureStep): string {
  return step ? (kindWords[step.kind] ?? step.kind) : "";
}

/*
 * Rules shows a step's rule where the comparison touches it: the old one
 * struck through when it changed, the new one when it changed or came
 * with an added step.
 */
function Rules({ change: c }: { change: StepChange }) {
  const [before, after] = [ruleLine(c.before), ruleLine(c.after)];
  const changed = c.change === "changed" && before !== after;

  return (
    <>
      {changed && before ? <span className={was}>Rule: {before}</span> : null}
      {after && (changed || c.change === "added") ? <span className={now}>Rule: {after}</span> : null}
    </>
  );
}

/*
 * Change is one step that differs: its number, how it differs, and its
 * words before and after. Only what changed is shown twice.
 */
function Change({ change: c, sources }: { change: StepChange; sources: ChipSources }) {
  if (c.change === "same") {
    return null;
  }
  const mark = changes[c.change];
  const before = c.before ? stepTitle(c.before, sources) : "";
  const after = c.after ? stepTitle(c.after, sources) : "";
  const kinds = [kindOf(c.before), kindOf(c.after)];
  const retyped = c.change === "changed" && kinds[0] !== kinds[1];

  return (
    <div className={line}>
      <span className="flex items-center gap-2">
        <span className="text-body-2-medium text-text-tertiary tabular-nums">Step {c.label}</span>
        <Chip variant="caption" color={mark.color} className="py-0.5">
          {mark.label}
        </Chip>
        <span className="text-caption-1-medium text-text-tertiary">
          {retyped ? `${kinds[0]} to ${kinds[1]}` : kinds[1] || kinds[0]}
        </span>
      </span>
      {before && before !== after ? <span className={c.after ? was : now}>{before}</span> : null}
      {after ? <span className={now}>{after}</span> : null}
      <Rules change={c} />
    </div>
  );
}

function Diff({
  before,
  after,
  sources,
}: {
  before: ProcedureDefinition;
  after: ProcedureDefinition;
  sources: ChipSources;
}) {
  const differing = diffSteps(before, after).filter((c) => c.change !== "same");
  const described = before.description !== after.description;

  if (differing.length === 0 && !described) {
    return <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">No differences.</p>;
  }

  return (
    <div className="flex flex-col">
      {described ? (
        <div className={line}>
          <span className="flex items-center gap-2">
            <span className="text-body-2-medium text-text-tertiary">When dimmi uses this</span>
            <Chip variant="caption" color="blue" className="py-0.5">
              Changed
            </Chip>
          </span>
          {before.description ? <span className={was}>{before.description}</span> : null}
          {after.description ? <span className={now}>{after.description}</span> : null}
        </div>
      ) : null}
      {differing.map((c) => (
        <Change key={c.id} change={c} sources={sources} />
      ))}
    </div>
  );
}

function VersionMenu({
  version,
  live,
  onCompare,
  onRestore,
  onPublish,
}: {
  version: number;
  live: boolean;
  onCompare: () => void;
  onRestore: () => void;
  onPublish: () => void;
}) {
  const [open, setOpen] = useState(false);
  const pick = (act: () => void) => () => {
    setOpen(false);
    act();
  };

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label={`Version ${version} options`}
        className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
      >
        <RiMore2Line aria-hidden className="size-4" />
      </DropdownTrigger>
      <DropdownPopover aria-label={`Version ${version} options`} placement="bottom end" className="w-60">
        <DropdownItem onSelect={pick(onCompare)}>Compare with working copy</DropdownItem>
        <DropdownItem onSelect={pick(onRestore)}>Restore as working copy</DropdownItem>
        {live ? null : <DropdownItem onSelect={pick(onPublish)}>Publish this version</DropdownItem>}
      </DropdownPopover>
    </Dropdown>
  );
}

export function ProcedureHistory({
  workspace,
  draft,
  sources,
  readOnly,
}: {
  workspace: string;
  draft: Draft;
  sources: ChipSources;
  readOnly: boolean;
}) {
  const id = draft.server.id;
  const versions = useQuery(versionsQuery(workspace, id));
  const list = versions.data ?? [];
  const [left, setLeft] = useState<string>("");
  const [right, setRight] = useState<string>(working);
  const [publishing, setPublishing] = useState<ProcedureVersion | null>(null);
  const choices: Choices = [
    [working, "Working copy"],
    ...list.map((v) => [String(v.version), `Version ${v.version}`] as const),
  ];
  const pick = (key: string): ProcedureDefinition | undefined =>
    key === working ? draft.definition : list.find((v) => String(v.version) === key)?.definition;
  const leftKey = left || (list[0] ? String(list[0].version) : "");
  const before = pick(leftKey);
  const after = pick(right);
  const liveId = draft.server.status === "live" ? list[0]?.id : undefined;

  const restore = async (v: ProcedureVersion, publish: boolean) => {
    try {
      await draft.flush();
      const next = await restoreVersion(workspace, id, v.version, { publish, lock_version: draft.server.lock_version });
      draft.adopt(next);
      void versions.refetch();
      toast.success(publish ? `Version ${v.version} is live again.` : `Version ${v.version} is the working copy.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The version could not be restored.");
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <Section
        title="Compare"
        action={
          list.length === 0 ? null : (
            <span className="flex items-center gap-1.5">
              <Pick key={`l${leftKey}`} label="Compare from" choices={choices} value={leftKey} onChange={setLeft} />
              <RiArrowRightLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
              <Pick key={`r${right}`} label="Compare to" choices={choices} value={right} onChange={setRight} />
            </span>
          )
        }
      >
        {before && after ? (
          <Diff before={before} after={after} sources={sources} />
        ) : (
          <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">
            Nothing to compare until a version is published.
          </p>
        )}
      </Section>
      <Section title="Versions" loading={versions.isPending}>
        {list.length === 0 ? (
          <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">Publish to keep the first version.</p>
        ) : null}
        {list.map((v) => (
          <div key={v.id} className="flex items-start gap-3 border-b border-separator-border py-3 pe-2 last:border-b-0">
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="flex items-center gap-2">
                <span className="text-body-2-medium text-text-primary">Version {v.version}</span>
                {v.id === liveId ? (
                  <Chip variant="caption" color="lime" className="py-0.5">
                    Live
                  </Chip>
                ) : null}
                <span className="text-body-2-regular text-text-tertiary" title={formatDateTime(v.published_at)}>
                  {formatRelative(v.published_at)}
                </span>
              </span>
              {v.change_note ? <span className="text-body-2-regular text-text-secondary">{v.change_note}</span> : null}
              <span className="text-caption-1-medium text-text-tertiary tabular-nums">
                {countOf(v.conversations, "conversation", "conversations")} · {v.handed_off} handed over
              </span>
            </div>
            {readOnly ? null : (
              <VersionMenu
                version={v.version}
                live={v.id === liveId}
                onCompare={() => {
                  setLeft(String(v.version));
                  setRight(working);
                }}
                onRestore={() => void restore(v, false)}
                onPublish={() => setPublishing(v)}
              />
            )}
          </div>
        ))}
      </Section>
      <AlertDialog
        isOpen={publishing !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPublishing(null);
          }
        }}
        variant="default"
        title={publishing ? `Publish version ${publishing.version}?` : "Publish this version?"}
        description="dimmi follows it at once in every conversation that matches. Your working copy is replaced by it."
        confirmLabel="Publish"
        onConfirm={() => {
          const v = publishing;
          setPublishing(null);
          if (v) {
            void restore(v, true);
          }
        }}
      />
    </div>
  );
}
