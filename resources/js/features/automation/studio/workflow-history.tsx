/*
 * A workflow's History, laid out like an agent's. Compare shows what the
 * working copy changes against the live version, or any two versions:
 * the trigger and rules above the steps, then each step added, removed,
 * moved or edited, a message's text line by line. Below, every version
 * published, newest first, with who published it, how, and the note it
 * went out with. Restoring one makes it the working copy, ready to check
 * and publish again; the live version keeps running until then.
 */
import { RiArrowRightLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Action } from "@/features/settings/controls";
import { Section } from "@/features/settings/layout";
import {
  restoreWorkflowVersion,
  type Spec,
  type Workflow,
  type WorkflowVersion,
  workflowQuery,
  workflowVersionsQuery,
} from "@/lib/api/workflows";
import { membersQuery } from "@/lib/api/workspace";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import type { Choices } from "../choices";
import { Pick } from "../pick";
import { type ChangeMark, type WorkflowChange, workflowChanges } from "./workflow-numbers";

const working = "working";
const line = "flex flex-col gap-1.5 border-b border-separator-border py-3 pe-3 last:border-b-0";
const was = "text-body-2-regular text-text-tertiary line-through";
const now = "text-body-2-regular text-text-primary";
const marks = { same: "text-text-tertiary", added: "bg-background-success-subtle text-text-primary", removed: was };

const markLook: Record<ChangeMark, { label: string; color: ChipColor }> = {
  added: { label: "Added", color: "lime" },
  removed: { label: "Removed", color: "rose" },
  changed: { label: "Changed", color: "blue" },
  moved: { label: "Moved", color: "purple" },
};

const via: Record<string, string> = { operator: "by Operator" };

/* Text shows only the lines that changed, with one kept line around each. */
function Text({ change }: { change: WorkflowChange }) {
  const lines = change.lines ?? [];
  const near = (i: number) => lines.slice(Math.max(0, i - 1), i + 2).some((l) => l.mark !== "same");

  return (
    <div className="flex flex-col rounded-lg bg-background-secondary-default p-2 font-mono text-caption-1-regular">
      {lines.map((l, i) =>
        l.mark === "same" && !near(i) ? null : (
          // biome-ignore lint/suspicious/noArrayIndexKey: lines repeat, their place tells them apart
          <span key={i} className={cx("whitespace-pre-wrap rounded px-1", marks[l.mark])}>
            {l.mark === "added" ? "+ " : l.mark === "removed" ? "- " : "  "}
            {l.text || " "}
          </span>
        ),
      )}
    </div>
  );
}

function Changes({ before, after }: { before: Spec; after: Spec }) {
  const changes = workflowChanges(before, after);
  if (changes.length === 0) {
    return <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">No differences.</p>;
  }

  return (
    <div className="flex flex-col">
      {changes.map((c) => (
        <div key={c.key} className={line}>
          <span className="flex min-w-0 items-center gap-2">
            <span className="truncate text-body-2-medium text-text-tertiary">{c.label}</span>
            <Chip variant="caption" color={markLook[c.mark].color} className="py-0.5">
              {markLook[c.mark].label}
            </Chip>
          </span>
          {c.lines ? <Text change={c} /> : null}
          {!c.lines && c.mark === "changed" && c.before !== undefined ? (
            <>
              <span className={was}>{c.before}</span>
              <span className={now}>{c.after}</span>
            </>
          ) : null}
          {c.mark === "moved" ? (
            <span className="text-body-2-regular text-text-secondary">{`From ${c.before} to ${c.after}`}</span>
          ) : null}
          {c.mark === "added" && c.after ? (
            <span className="text-body-2-regular text-text-secondary">{`Under ${c.after}`}</span>
          ) : null}
        </div>
      ))}
    </div>
  );
}

/*
 * Compare picks two copies, the live version and the working copy at
 * first, and lists what differs between them.
 */
function Compare({
  workflow,
  versions,
  loading,
}: {
  workflow?: Workflow;
  versions: WorkflowVersion[];
  loading: boolean;
}) {
  const live = versions.find((v) => v.id === workflow?.live_version_id)?.number ?? 0;
  const [left, setLeft] = useState("");
  const [right, setRight] = useState(working);
  const label = (v: WorkflowVersion) => (v.number === live ? `Version ${v.number} (live)` : `Version ${v.number}`);
  const choices: Choices = [[working, "Working copy"], ...versions.map((v) => [String(v.number), label(v)] as const)];
  const pick = (key: string): Spec | undefined =>
    key === working ? workflow?.spec : versions.find((v) => String(v.number) === key)?.spec;
  const leftKey = left || String(live || versions[0]?.number || "");
  const [before, after] = [pick(leftKey), pick(right)];

  return (
    <Section
      title="Compare"
      loading={loading}
      rows={2}
      action={
        versions.length === 0 ? null : (
          <span className="flex items-center gap-1.5">
            <Pick key={`l${leftKey}`} label="Compare from" choices={choices} value={leftKey} onChange={setLeft} />
            <RiArrowRightLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
            <Pick key={`r${right}`} label="Compare to" choices={choices} value={right} onChange={setRight} />
          </span>
        )
      }
    >
      {before && after ? (
        <Changes before={before} after={after} />
      ) : (
        <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">
          Nothing to compare until a version is published.
        </p>
      )}
    </Section>
  );
}

export function WorkflowHistory({
  workspace,
  workflow,
  onRestored,
}: {
  workspace: string;
  workflow: string;
  /* Hands the restored working copy to an open editor, so it adopts it. */
  onRestored?: (restored: Workflow) => void;
}) {
  const client = useQueryClient();
  const { data: wf } = useQuery(workflowQuery(workspace, workflow));
  const { data: versions = [], isPending } = useQuery(workflowVersionsQuery(workspace, workflow));
  const members = useQuery(membersQuery(workspace)).data ?? [];
  const [restoring, setRestoring] = useState<WorkflowVersion | null>(null);
  const names = new Map(members.map((m) => [m.id, m.user.name]));
  const live = versions.find((v) => v.id === wf?.live_version_id)?.number ?? 0;
  const restoredFrom = new Map(versions.map((v) => [v.id, v.number]));

  const restore = async (version: WorkflowVersion) => {
    try {
      const restored = await restoreWorkflowVersion(workspace, workflow, version.id);
      client.setQueryData(workflowQuery(workspace, workflow).queryKey, restored);
      void client.invalidateQueries({ queryKey: ["workspace", workspace, "workflows"] });
      onRestored?.(restored);
      toast.success(`Version ${version.number} is your working copy`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The version was not restored.");
    }
  };

  const byline = (v: WorkflowVersion) =>
    [
      v.published_by_member_id ? `By ${names.get(v.published_by_member_id) ?? "a teammate"}` : "",
      v.published_via && via[v.published_via] ? via[v.published_via] : "",
      v.restored_from_version_id && restoredFrom.has(v.restored_from_version_id)
        ? `restored from version ${restoredFrom.get(v.restored_from_version_id)}`
        : "",
      formatRelative(v.created_at),
    ]
      .filter(Boolean)
      .join(" · ");

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <Compare workflow={wf} versions={versions} loading={isPending} />
      <Section title="Versions" loading={isPending} rows={2}>
        {versions.length === 0 ? (
          <p className="py-3 pe-3 text-body-2-regular text-text-secondary">
            Nothing published yet. Each publish keeps a version you can come back to.
          </p>
        ) : (
          versions.map((v) => (
            <div
              key={v.id}
              className="flex items-start gap-3 border-b border-separator-border py-3 pe-2 last:border-b-0"
            >
              <div className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="flex items-center gap-2">
                  <span className="text-body-2-medium text-text-primary">Version {v.number}</span>
                  {v.number === live ? (
                    <Chip variant="caption" color="lime" className="py-0.5">
                      Live
                    </Chip>
                  ) : null}
                </span>
                <span className="text-body-2-regular text-text-tertiary">{byline(v)}</span>
                {v.note ? <span className="text-body-2-regular text-text-secondary">{v.note}</span> : null}
              </div>
              {v.number === live && !wf?.has_unpublished_changes ? null : (
                <Action onPress={() => setRestoring(v)}>Restore</Action>
              )}
            </div>
          ))
        )}
      </Section>
      <AlertDialog
        isOpen={restoring !== null}
        onOpenChange={(open) => (open ? undefined : setRestoring(null))}
        variant="default"
        icon={false}
        title={restoring ? `Restore version ${restoring.number}?` : "Restore version?"}
        description="It replaces your working copy. Conversations keep the live version until you publish."
        confirmLabel="Restore"
        onConfirm={() => {
          const v = restoring;
          setRestoring(null);

          return v ? restore(v) : undefined;
        }}
      />
    </div>
  );
}
