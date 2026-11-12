/*
 * The changelog's own pane beside its list, the way the help center has
 * its collections: every update or those in one state, each label, and
 * the people subscribed. Labels are added, renamed, recolored and
 * deleted right here, the same way collections are: a plus to add one,
 * the name typed in its row, a menu on hover for the rest.
 */
import {
  RiAddLine,
  RiCalendarScheduleLine,
  RiDraftLine,
  RiMailLine,
  RiMegaphoneLine,
  RiSendPlaneLine,
} from "@remixicon/react";
import { type ComponentType, useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { InlineName } from "@/features/help-center/inline-name";
import { Panel, ScrollArea } from "@/features/shell/page";
import { RowMenu } from "@/features/shell/row-menu";
import { type ChangelogLabel, type EntryStatus, labelColors } from "@/lib/api/changelog";
import { cx } from "@/utils/cx";

import { type LabelAct, labelActions, useLabels } from "./use-labels";

/*
 * View is what the list shows: updates in a state or under a label, or
 * the subscribers.
 */
export type View =
  | { kind: "all" }
  | { kind: "status"; status: EntryStatus }
  | { kind: "label"; id: string }
  | { kind: "subscribers" };

const statusPicks: { status: EntryStatus; label: string; icon: ComponentType<{ className?: string }> }[] = [
  { status: "draft", label: "Drafts", icon: RiDraftLine },
  { status: "scheduled", label: "Scheduled", icon: RiCalendarScheduleLine },
  { status: "published", label: "Published", icon: RiSendPlaneLine },
];

function rowClass(current: boolean): string {
  return cx(
    "group/row flex min-h-8 w-full items-center gap-2 rounded-xl ps-3 pe-2 text-text-primary outline-none",
    "transition-[background-color,box-shadow] duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
    current
      ? "bg-background-primary-default text-body-2-medium shadow-xs ring-1 ring-inset ring-border-button-default"
      : "text-body-2-regular hover:bg-background-secondary-hover",
  );
}

function Pick({
  label,
  icon: Icon,
  count,
  current,
  onPress,
}: {
  label: string;
  icon: ComponentType<{ className?: string }>;
  count?: number;
  current: boolean;
  onPress: () => void;
}) {
  return (
    <button type="button" aria-current={current ? "page" : undefined} onClick={onPress} className={rowClass(current)}>
      <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary opacity-50" />
      <span className="min-w-0 flex-1 truncate text-start">{label}</span>
      {count ? <span className="text-caption-1-regular text-text-tertiary tabular-nums">{count}</span> : null}
    </button>
  );
}

function LabelRow({
  label,
  current,
  renaming,
  onPick,
  onAct,
  onName,
  onCancel,
}: {
  label: ChangelogLabel;
  current: boolean;
  renaming: boolean;
  onPick: () => void;
  onAct: (act: LabelAct) => void;
  onName: (name: string) => Promise<unknown>;
  onCancel: () => void;
}) {
  if (renaming) {
    return (
      <div className="flex min-h-8 items-center gap-2 ps-3 pe-2">
        <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ background: label.color }} />
        <InlineName initial={label.name} label="Label name" onCommit={onName} onCancel={onCancel} />
      </div>
    );
  }
  return (
    <div className={cx(rowClass(current), "pe-1")}>
      <button
        type="button"
        aria-current={current ? "page" : undefined}
        onClick={onPick}
        className="flex min-w-0 flex-1 items-center gap-2 self-stretch outline-none"
      >
        <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ background: label.color }} />
        <span className="min-w-0 flex-1 truncate text-start">{label.name}</span>
      </button>
      <span className="opacity-0 transition-opacity group-focus-within/row:opacity-100 group-hover/row:opacity-100 has-[[aria-expanded=true]]:opacity-100 pointer-coarse:opacity-100">
        <RowMenu quiet label={label.name} actions={labelActions} onPick={onAct} />
      </span>
    </div>
  );
}

export function ChangelogPanel({
  workspace,
  helpCenter,
  labels,
  counts,
  view,
  onView,
}: {
  workspace: string;
  helpCenter: string;
  labels: ChangelogLabel[];
  counts: Partial<Record<EntryStatus, number>>;
  view: View;
  onView: (view: View) => void;
}) {
  const edits = useLabels(workspace, helpCenter);
  const [adding, setAdding] = useState(false);
  const [renaming, setRenaming] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<ChangelogLabel | null>(null);
  const total = (counts.draft ?? 0) + (counts.scheduled ?? 0) + (counts.published ?? 0);
  const onAct = (label: ChangelogLabel, act: LabelAct) => {
    if (act === "rename") {
      setRenaming(label.id);
    } else if (act === "color") {
      edits.recolor(label);
    } else {
      setDeleting(label);
    }
  };

  return (
    <Panel label="Changelog views" className="h-full w-full">
      <header className="flex h-14 shrink-0 items-center gap-2 ps-4 pe-2.5">
        <h2 className="truncate text-body-medium text-text-primary">Changelog</h2>
      </header>
      <ScrollArea>
        <nav aria-label="Changelog views" className="flex flex-col gap-0.5 px-2 pb-4">
          <Pick
            label="All updates"
            icon={RiMegaphoneLine}
            count={total}
            current={view.kind === "all"}
            onPress={() => onView({ kind: "all" })}
          />
          {statusPicks.map((p) => (
            <Pick
              key={p.status}
              label={p.label}
              icon={p.icon}
              count={counts[p.status]}
              current={view.kind === "status" && view.status === p.status}
              onPress={() => onView({ kind: "status", status: p.status })}
            />
          ))}
          <div className="mt-4 flex h-7 items-center justify-between ps-3 pe-1">
            <h3 className="text-caption-1-semibold text-text-tertiary">Labels</h3>
            <IconButton
              size="small"
              icon={RiAddLine}
              aria-label="New label"
              onClick={() => setAdding(true)}
              className="size-6 border-transparent bg-transparent shadow-none"
            />
          </div>
          {labels.map((l) => (
            <LabelRow
              key={l.id}
              label={l}
              current={view.kind === "label" && view.id === l.id}
              renaming={renaming === l.id}
              onPick={() => onView({ kind: "label", id: l.id })}
              onAct={(act) => onAct(l, act)}
              onName={async (name) => {
                await edits.rename(l.id, name);
                setRenaming(null);
              }}
              onCancel={() => setRenaming(null)}
            />
          ))}
          {adding ? (
            <div className="flex min-h-8 items-center gap-2 ps-3 pe-2">
              <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ background: labelColors[3] }} />
              <InlineName
                initial=""
                label="New label name"
                onCommit={async (name) => {
                  await edits.add(name);
                  setAdding(false);
                }}
                onCancel={() => setAdding(false)}
              />
            </div>
          ) : null}
          {labels.length === 0 && !adding ? (
            <p className="px-3 py-1 text-body-2-regular text-text-tertiary">No labels yet.</p>
          ) : null}
          <div aria-hidden className="mx-3 my-3 h-px bg-separator-border" />
          <Pick
            label="Subscribers"
            icon={RiMailLine}
            current={view.kind === "subscribers"}
            onPress={() => onView({ kind: "subscribers" })}
          />
        </nav>
      </ScrollArea>
      <AlertDialog
        isOpen={deleting !== null}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
        title={deleting ? `Delete ${deleting.name}?` : "Delete label?"}
        description="Updates filed under it stay, without this label."
        confirmLabel="Delete label"
        onConfirm={async () => {
          if (deleting) {
            await edits.remove(deleting.id);
            if (view.kind === "label" && view.id === deleting.id) {
              onView({ kind: "all" });
            }
          }
        }}
      />
    </Panel>
  );
}
