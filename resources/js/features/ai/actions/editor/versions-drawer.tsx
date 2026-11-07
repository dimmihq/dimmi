/*
 * The versions of an action, newest first. Each publish freezes the
 * action as a numbered version that never changes, and every call
 * records the version it ran. Restoring an old version copies it into
 * the draft; customers keep the live version until it is published, so
 * going back is always one restore and one publish.
 */
import { RiCloseLine, RiHistoryLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { ago } from "@/features/inbox/conversations";
import { RowsSkeleton } from "@/features/settings/layout";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";

import { type ActionVersion, type AiAction, versionsQuery } from "../api";
import { permissionSummary } from "../format";

/*
 * specOf reads what a version froze that its row shows: the title and
 * who had to agree.
 */
function specOf(version: ActionVersion) {
  const spec = version.spec as Partial<Pick<AiAction, "title" | "name" | "side_effect" | "confirmation" | "approval">>;

  return {
    title: spec.title || spec.name || `Version ${version.number}`,
    side_effect: spec.side_effect ?? "read",
    confirmation: spec.confirmation ?? "none",
    approval: spec.approval ?? "none",
  };
}

function VersionRow({
  version,
  live,
  onRestore,
}: {
  version: ActionVersion;
  live: boolean;
  onRestore: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const by = version.published_by?.name;
  const spec = specOf(version);

  return (
    <li className="flex items-start gap-3 border-b border-separator-border py-3 pe-3 last:border-b-0">
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-background-tertiary-default text-body-2-medium text-text-primary tabular-nums">
        v{version.number}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-center gap-2 text-body-regular text-text-primary">
          {spec.title}
          {live ? (
            <Chip variant="caption" color="lime">
              Live
            </Chip>
          ) : null}
        </span>
        <span className="text-body-2-regular text-text-secondary">
          {[by ? `Published by ${by}` : "Published", ago(version.created_at).toLowerCase()].join(" · ")}
        </span>
        <span className="text-body-2-regular text-text-tertiary">{permissionSummary(spec)}</span>
        {version.note ? <span className="text-body-2-regular text-text-secondary">“{version.note}”</span> : null}
      </div>
      <Button
        variant="secondary"
        size="xs"
        loading={busy}
        onClick={async () => {
          setBusy(true);
          await onRestore();
          setBusy(false);
        }}
      >
        Restore
      </Button>
    </li>
  );
}

export function VersionsDrawer({
  workspace,
  action,
  isOpen,
  onOpenChange,
  onRestore,
}: {
  workspace: string;
  action: AiAction;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onRestore: (version: ActionVersion) => Promise<void>;
}) {
  const versions = useQuery({ ...versionsQuery(workspace, action.id), enabled: isOpen });
  const list = [...(versions.data ?? [])].sort((a, b) => b.number - a.number);

  return (
    <Drawer isOpen={isOpen} onOpenChange={onOpenChange} placement="end" size="md" aria-label="Versions" className="p-0">
      <Panel label="Versions" ground className="h-full">
        <PageHeader title="Versions" level={2}>
          <IconButton size="small" icon={RiCloseLine} aria-label="Close versions" onClick={() => onOpenChange(false)} />
        </PageHeader>
        <ScrollArea ground className="px-4 pb-6">
          {versions.isPending ? (
            <div className="rounded-2xl bg-background-secondary-default ps-3">
              <RowsSkeleton rows={3} />
            </div>
          ) : null}
          {versions.data && list.length === 0 ? (
            <EmptyState
              icon={RiHistoryLine}
              title="Not published yet"
              description="Each publish saves a version you can come back to."
            />
          ) : null}
          {list.length > 0 ? (
            <ul aria-label="Versions" className="rounded-2xl bg-background-secondary-default ps-3">
              {list.map((version) => (
                <VersionRow
                  key={version.id}
                  version={version}
                  live={action.published_version?.id === version.id && action.status === "published"}
                  onRestore={() => onRestore(version)}
                />
              ))}
            </ul>
          ) : null}
        </ScrollArea>
      </Panel>
    </Drawer>
  );
}
