/*
 * A series' History: every version published, newest first, with who
 * published it and the note it went out with, the live one marked.
 * Restoring one copies it into the working copy, ready to check and
 * publish again; people on the series keep the live version until then.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Action } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { restoreSeriesVersion, type Series, type SeriesVersion, seriesVersionsQuery } from "@/lib/api/series";
import { membersQuery } from "@/lib/api/workspace";
import { formatRelative } from "@/lib/i18n/format";

export function SeriesHistory({
  workspace,
  series,
  onAdopt,
}: {
  workspace: string;
  series: Series;
  onAdopt?: (next: Series) => void;
}) {
  const { data: versions = [], isPending } = useQuery(seriesVersionsQuery(workspace, series.id));
  const members = useQuery(membersQuery(workspace)).data ?? [];
  const names = new Map(members.map((m) => [m.id, m.user.name]));
  const [restoring, setRestoring] = useState<SeriesVersion | null>(null);

  return (
    <>
      <Section
        title="Versions"
        description="Every version published, newest first. Restoring one makes it your working copy."
        loading={isPending}
      >
        {versions.length === 0 && !isPending ? (
          <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">Nothing published yet.</p>
        ) : null}
        {versions.map((v) => (
          <Row
            key={v.id}
            label={
              <span className="flex items-center gap-2">
                Version {v.number}
                {v.id === series.live_version_id ? (
                  <Chip variant="caption" color="lime">
                    Live
                  </Chip>
                ) : null}
              </span>
            }
            description={[
              v.note,
              `${v.published_by ? (names.get(v.published_by) ?? "A teammate") : "Someone"} · ${formatRelative(v.created_at)}`,
            ]
              .filter(Boolean)
              .join(" · ")}
          >
            <Action onPress={() => setRestoring(v)}>Restore</Action>
          </Row>
        ))}
      </Section>
      <AlertDialog
        isOpen={restoring !== null}
        onOpenChange={(open) => (open ? null : setRestoring(null))}
        variant="default"
        title={restoring ? `Restore version ${restoring.number}?` : "Restore this version?"}
        description="It replaces your working copy. People on the series keep the live version until you publish."
        confirmLabel="Restore"
        onConfirm={async () => {
          if (!restoring) {
            return;
          }
          try {
            const next = await restoreSeriesVersion(workspace, series.id, restoring.id);
            onAdopt?.(next);
            toast.success(`Version ${restoring.number} is your working copy`);
            setRestoring(null);
          } catch (error) {
            toast.error(error instanceof Error ? error.message : "The version could not be restored.");
            throw error;
          }
        }}
      />
    </>
  );
}
