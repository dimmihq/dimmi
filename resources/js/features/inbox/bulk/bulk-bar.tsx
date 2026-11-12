/*
 * The bar that replaces the list's controls while conversations are
 * picked: how many, a way to pick every row loaded or none, and the
 * changes that apply to all of them at once. Each conversation changes
 * on its own on the server, so one that cannot change, such as one a
 * teammate just trashed, is counted as skipped and the rest go through.
 * A change that takes conversations out of the list empties the pick.
 */
import {
  RiAlarmLine,
  RiCheckboxCircleLine,
  RiCloseLine,
  RiDeleteBinLine,
  RiMailCheckLine,
  RiMailUnreadLine,
  RiRefreshLine,
  RiSpam2Line,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { toast } from "@/components/base/toast/toast";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import { type BulkChange, bulkChange, type ConversationChanges } from "@/lib/api/inbox";
import { callerQuery } from "@/lib/api/workspace";
import { useMutationId } from "@/lib/realtime/context";
import { cx } from "@/utils/cx";

import { AssignPicker } from "../assign/assign-picker";
import { TagPicker } from "../tags/tag-picker";
import type { SelectionControls } from "./selection";

const snoozes: { preset: NonNullable<ConversationChanges["snooze_preset"]>; label: string }[] = [
  { preset: "later_today", label: "Later today" },
  { preset: "tomorrow", label: "Tomorrow" },
  { preset: "next_week", label: "Next week" },
  { preset: "one_week", label: "In one week" },
];

type More = "read" | "unread" | "spam" | "trash";

/*
 * done says what a bulk change did, in the past tense, by what it sent.
 */
function done(change: BulkChange): string {
  if ("trash" in change) {
    return "Moved to the trash";
  }
  const c = change.changes;
  const said: [boolean, string][] = [
    [c.status === "closed", "Closed"],
    [c.status === "open", "Reopened"],
    [c.status === "snoozed", "Snoozed"],
    [c.spam === true, "Marked as spam"],
    [c.unread === true, "Marked as unread"],
    [c.unread === false, "Marked as read"],
    [c.add_tags !== undefined, "Tagged"],
  ];

  return said.find(([hit]) => hit)?.[1] ?? "Assigned";
}

/*
 * leaves reports whether the change takes the conversations out of the
 * list, so the pick is emptied.
 */
function leaves(change: BulkChange): boolean {
  return "trash" in change || change.changes.status !== undefined || change.changes.spam === true;
}

function plural(n: number): string {
  return n === 1 ? "1 conversation" : `${n} conversations`;
}

export function BulkBar({
  workspace,
  selection,
  loaded,
}: {
  workspace: string;
  selection: SelectionControls;
  loaded: number;
}) {
  const client = useQueryClient();
  const mutationId = useMutationId();
  const [assigning, setAssigning] = useState(false);
  const [snoozing, setSnoozing] = useState(false);
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const canTrash = caller?.member?.permissions.includes("conversations:delete") ?? false;
  const ids = [...selection.ids];

  const run = useMutation({
    mutationFn: (change: BulkChange) => bulkChange({ workspace, mutation: mutationId() }, ids, change),
    onSuccess: (results, change) => {
      const changed = results.filter((r) => r.ok).length;
      const skipped = results.length - changed;
      const text = `${done(change)} ${plural(changed)}`;
      if (skipped > 0) {
        toast.error(`${text}. ${plural(skipped)} could not be changed.`);
      } else {
        toast.success(text);
      }
      if (leaves(change)) {
        selection.clear();
      }
      return client.invalidateQueries({ queryKey: ["workspace", workspace] });
    },
    onError: () => toast.error("The conversations could not be changed."),
  });
  const apply = (changes: ConversationChanges) => run.mutate({ changes });

  const more: RowAction<More>[] = [
    { kind: "read", label: "Mark as read", icon: RiMailCheckLine },
    { kind: "unread", label: "Mark as unread", icon: RiMailUnreadLine },
    { kind: "spam", label: "Mark as spam", icon: RiSpam2Line, danger: true },
    ...(canTrash ? [{ kind: "trash" as const, label: "Move to trash", icon: RiDeleteBinLine, danger: true }] : []),
  ];
  const pickMore = (kind: More) => {
    if (kind === "trash") {
      run.mutate({ trash: true });
    } else if (kind === "spam") {
      apply({ spam: true });
    } else {
      apply({ unread: kind === "unread" });
    }
  };

  return (
    <section
      aria-label="Bulk actions"
      aria-busy={run.isPending}
      className={cx("flex shrink-0 flex-col gap-2 px-[7px] pb-2", run.isPending && "opacity-70")}
    >
      <div className="flex min-w-0 items-center gap-2 ps-1.5">
        <span className="min-w-0 truncate text-body-2-medium text-text-primary" aria-live="polite">
          {selection.count} selected
        </span>
        {selection.count < loaded ? (
          <Button size="xs" variant="ghost" onClick={selection.selectAll}>
            Select all {loaded}
          </Button>
        ) : null}
        <span className="flex-1" />
        <IconButton size="small" icon={RiCloseLine} aria-label="Clear selection" onClick={selection.clear} />
      </div>
      <div className="flex min-w-0 flex-wrap items-center gap-1">
        <IconButton
          size="small"
          icon={RiCheckboxCircleLine}
          aria-label="Close selected"
          onClick={() => apply({ status: "closed" })}
        />
        <IconButton
          size="small"
          icon={RiRefreshLine}
          aria-label="Reopen selected"
          onClick={() => apply({ status: "open" })}
        />
        <Dropdown isOpen={snoozing} onOpenChange={setSnoozing}>
          <DropdownTrigger
            aria-label="Snooze selected"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
          >
            <RiAlarmLine aria-hidden className="size-4" />
          </DropdownTrigger>
          <DropdownPopover aria-label="Snooze until" placement="bottom start" className="w-48">
            {snoozes.map((s) => (
              <DropdownItem
                key={s.preset}
                onSelect={() => {
                  setSnoozing(false);
                  apply({ status: "snoozed", snooze_preset: s.preset });
                }}
              >
                {s.label}
              </DropdownItem>
            ))}
          </DropdownPopover>
        </Dropdown>
        <AssignPicker
          workspace={workspace}
          assignee={null}
          team={null}
          isOpen={assigning}
          onOpenChange={setAssigning}
          onMember={(id) => apply({ assignee_id: id })}
          onTeam={(id) => apply({ team_id: id })}
        />
        <TagPicker
          workspace={workspace}
          selected={[]}
          label="Tag selected"
          onAdd={(name) => apply({ add_tags: [name] })}
          onRemove={() => undefined}
        />
        <span className="flex-1" />
        <RowMenu label="the selected conversations" actions={more} onPick={pickMore} />
      </div>
    </section>
  );
}
