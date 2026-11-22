/*
 * An agent's History, laid out like a procedure's: every version it
 * published, newest first, with the note it went out with. Restoring
 * one makes it the working copy, ready to check and publish again.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Action } from "@/features/settings/controls";
import { Section } from "@/features/settings/layout";
import { type AgentVersion, agentVersionsQuery, restoreAgentVersion } from "@/lib/api/agents";
import { formatRelative } from "@/lib/i18n/format";

import type { AgentDraft } from "./use-agent-draft";

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
