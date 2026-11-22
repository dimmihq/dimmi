/*
 * An agent's History, laid out like a procedure's. Compare shows what
 * the working copy changes against the live version, or any two
 * versions, field by field and guidance line by line. Below, every
 * version it published, newest first, with the note it went out with.
 * Restoring one makes it the working copy, ready to check and publish
 * again.
 */
import { RiArrowRightLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import type { Choices } from "@/features/automation/choices";
import { Pick } from "@/features/automation/pick";
import { Action } from "@/features/settings/controls";
import { Section } from "@/features/settings/layout";
import {
  type AgentDefinition,
  type AgentVersion,
  agentsQuery,
  agentVersionsQuery,
  placementsQuery,
  restoreAgentVersion,
} from "@/lib/api/agents";
import { teamsQuery } from "@/lib/api/teams";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { agentChanges, type FieldChange, type Namer } from "./agent-diff";
import type { AgentDraft } from "./use-agent-draft";

const working = "working";
const line = "flex flex-col gap-1.5 border-b border-separator-border py-3 pe-3 last:border-b-0";
const was = "text-body-2-regular text-text-tertiary line-through";
const now = "text-body-2-regular text-text-primary";
const marks = { same: "text-text-tertiary", added: "bg-background-success-subtle text-text-primary", removed: was };

/* useNamer names the agents, places and teams an agent's fields point at. */
function useNamer(workspace: string): Namer {
  const agents = useQuery(agentsQuery(workspace)).data ?? [];
  const places = useQuery(placementsQuery(workspace)).data ?? [];
  const teams = useQuery(teamsQuery(workspace)).data ?? [];
  const names = new Map<string, string>([
    ...agents.map((a) => [a.id, a.name] as const),
    ...places.map((p) => [p.id, p.name] as const),
    ...teams.map((t) => [t.id, t.name] as const),
  ]);

  return (id) => names.get(id);
}

/* Guidance shows only the lines that changed, with one kept line around each. */
function Guidance({ change }: { change: FieldChange }) {
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

function Changes({ before, after, name }: { before: AgentDefinition; after: AgentDefinition; name: Namer }) {
  const changes = agentChanges(before, after, name);
  if (changes.length === 0) {
    return <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">No differences.</p>;
  }

  return (
    <div className="flex flex-col">
      {changes.map((c) => (
        <div key={c.key} className={line}>
          <span className="flex items-center gap-2">
            <span className="text-body-2-medium text-text-tertiary">{c.label}</span>
            <Chip variant="caption" color="blue" className="py-0.5">
              Changed
            </Chip>
          </span>
          {c.lines ? (
            <Guidance change={c} />
          ) : (
            <>
              <span className={was}>{c.before}</span>
              <span className={now}>{c.after}</span>
            </>
          )}
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
  workspace,
  draft,
  versions,
  loading,
}: {
  workspace: string;
  draft: AgentDraft;
  versions: AgentVersion[];
  loading: boolean;
}) {
  const name = useNamer(workspace);
  const [left, setLeft] = useState("");
  const [right, setRight] = useState(working);
  const live = draft.server.status === "live" ? draft.server.live_version : 0;
  const label = (v: AgentVersion) => (v.version === live ? `Version ${v.version} (live)` : `Version ${v.version}`);
  const choices: Choices = [[working, "Working copy"], ...versions.map((v) => [String(v.version), label(v)] as const)];
  const pick = (key: string): AgentDefinition | undefined =>
    key === working ? draft.draft : versions.find((v) => String(v.version) === key)?.definition;
  const leftKey = left || String(live || versions[0]?.version || "");
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
        <Changes before={before} after={after} name={name} />
      ) : (
        <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">
          Nothing to compare until a version is published.
        </p>
      )}
    </Section>
  );
}

export function AgentHistory({ workspace, draft }: { workspace: string; draft: AgentDraft }) {
  const { data: versions = [], isPending } = useQuery(agentVersionsQuery(workspace, draft.id));
  const [restoring, setRestoring] = useState<AgentVersion | null>(null);
  const live = draft.server.status === "live" ? draft.server.live_version : 0;

  const restore = async (version: AgentVersion) => {
    try {
      await draft.flush();
      draft.adopt(await restoreAgentVersion(workspace, draft.id, version.version, false));
      toast.success(`Version ${version.version} is your working copy`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The version was not restored.");
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <Compare workspace={workspace} draft={draft} versions={versions} loading={isPending} />
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
                  <span className="text-body-2-medium text-text-primary">Version {v.version}</span>
                  {v.version === live ? (
                    <Chip variant="caption" color="lime" className="py-0.5">
                      Live
                    </Chip>
                  ) : null}
                  <span className="text-body-2-regular text-text-tertiary">{formatRelative(v.published_at)}</span>
                </span>
                {v.change_note ? (
                  <span className="text-body-2-regular text-text-secondary">{v.change_note}</span>
                ) : null}
              </div>
              {v.version === live && !draft.server.has_unpublished_changes ? null : (
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
        title={restoring ? `Restore version ${restoring.version}?` : "Restore version?"}
        description="It replaces your working copy. Customers keep the live version until you publish."
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
