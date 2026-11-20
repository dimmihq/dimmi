/*
 * Saved replies and macros: the answers the team sends often, shared or
 * one person's own, with variables filled in for each customer and the
 * actions a macro applies when it is sent. Each is one short row: its
 * title, its shortcut, who can use it and whether it is a macro; the
 * row opens it to edit. Anyone keeps their own; sharing, and changing
 * what is shared, show only to people the server lets manage shared
 * replies.
 */
import { RiAddLine, RiDeleteBinLine, RiPencilLine, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { hasActions, macroSummary, type Names } from "@/features/saved-replies/macro";
import { ReplyDialog } from "@/features/saved-replies/reply-dialog";
import { useNames } from "@/features/saved-replies/use-names";
import { RowMenu } from "@/features/shell/row-menu";
import { deleteSavedReply, type SavedReply, savedRepliesKey, savedRepliesQuery } from "@/lib/api/saved-replies";
import { callerQuery } from "@/lib/api/workspace";
import { SettingsPage } from "../layout";
import { placeholders } from "../placeholders";
import { useAccount } from "../use-account";

function ReplyRow({
  reply,
  names,
  canChange,
  onEdit,
  onDelete,
}: {
  reply: SavedReply;
  names: Names;
  canChange: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const summary = hasActions(reply.actions) ? macroSummary(reply.actions, names) : [];

  return (
    <li className="group relative flex min-h-[52px] items-center gap-3 rounded-xl ps-3 pe-2 transition-colors duration-150 hover:bg-background-secondary-hover has-[:focus-visible]:bg-background-secondary-hover">
      <button
        type="button"
        disabled={!canChange}
        onClick={onEdit}
        aria-label={`Edit ${reply.title}`}
        className="flex min-w-0 flex-1 items-center gap-3 py-2.5 text-start outline-none enabled:cursor-pointer after:absolute after:inset-0 after:rounded-xl"
      >
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="flex min-w-0 items-center gap-2">
            <span className="truncate text-body-regular text-text-primary">{reply.title}</span>
            {reply.shortcut ? (
              <span className="shrink-0 font-mono text-body-2-regular text-text-tertiary">#{reply.shortcut}</span>
            ) : null}
          </span>
          <span className="truncate text-body-2-regular text-text-secondary">
            {[
              reply.shared ? "Shared with the team" : "Only you",
              summary.length > 0 ? `Then ${summary.join(", ")}` : null,
            ]
              .filter(Boolean)
              .join(" · ")}
          </span>
        </span>
        {summary.length > 0 ? (
          <Chip variant="caption" color="neutral" className="shrink-0">
            Macro
          </Chip>
        ) : null}
      </button>
      {canChange ? (
        <span className="relative z-10 shrink-0">
          <RowMenu
            label={reply.title}
            quiet
            actions={[
              { kind: "edit", label: "Edit", icon: RiPencilLine },
              { kind: "delete", label: "Delete", icon: RiDeleteBinLine, danger: true },
            ]}
            onPick={(kind) => (kind === "edit" ? onEdit() : onDelete())}
          />
        </span>
      ) : null}
    </li>
  );
}

function RowsSkeleton() {
  return placeholders(4).map((id) => (
    <li key={id} aria-hidden className="flex h-[52px] flex-col justify-center gap-1.5 px-3">
      <Skeleton shape="text" className="h-3 w-40" />
      <Skeleton shape="text" className="h-3 w-56" />
    </li>
  ));
}

function DeleteDialog({ workspace, reply, onClose }: { workspace: string; reply: SavedReply; onClose: () => void }) {
  const client = useQueryClient();
  const remove = useMutation({
    mutationFn: () => deleteSavedReply(workspace, reply.id),
    onSuccess: () => client.invalidateQueries({ queryKey: savedRepliesKey(workspace) }),
  });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Delete ${reply.title}?`}
      description={
        reply.shared
          ? "Nobody on the team can insert it any more. This cannot be undone."
          : "You can no longer insert it. This cannot be undone."
      }
      confirmLabel="Delete saved reply"
      onConfirm={() => remove.mutateAsync()}
    />
  );
}

export function SavedRepliesSettings() {
  const workspace = useAccount().workspace.id;
  const [search, setSearch] = useState("");
  const { data: replies } = useQuery(savedRepliesQuery(workspace, useDeferredValue(search)));
  const { data: caller } = useQuery(callerQuery(workspace));
  const names = useNames(workspace);
  const canShare = caller?.member?.permissions.includes("saved_replies:manage_shared") ?? false;
  const [editing, setEditing] = useState<SavedReply | "new" | null>(null);
  const [deleting, setDeleting] = useState<SavedReply | null>(null);

  return (
    <SettingsPage
      title="Saved replies"
      description="Answers your team sends often. Insert one from a reply's add menu, or type # and its shortcut."
    >
      <section aria-label="Saved replies" className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Input
            aria-label="Search saved replies"
            placeholder="Search saved replies"
            leadingIcon={RiSearchLine}
            value={search}
            onChange={setSearch}
            className="min-w-48 flex-1"
          />
          <Button leadingIcon={RiAddLine} onClick={() => setEditing("new")}>
            New saved reply
          </Button>
        </div>
        {replies?.length === 0 ? (
          <p className="rounded-2xl bg-background-secondary-default px-3 py-2.5 text-body-2-regular text-text-tertiary">
            {search.trim() ? "No saved replies match." : "No saved replies yet."}
          </p>
        ) : null}
        <ul
          aria-busy={!replies || undefined}
          className={
            replies?.length === 0 ? "hidden" : "flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1"
          }
        >
          {replies ? null : <RowsSkeleton />}
          {replies?.map((reply) => (
            <ReplyRow
              key={reply.id}
              reply={reply}
              names={names}
              canChange={!reply.shared || canShare}
              onEdit={() => setEditing(reply)}
              onDelete={() => setDeleting(reply)}
            />
          ))}
        </ul>
      </section>
      {editing ? (
        <ReplyDialog
          key={editing === "new" ? "new" : editing.id}
          workspace={workspace}
          reply={editing === "new" ? null : editing}
          canShare={canShare}
          onClose={() => setEditing(null)}
        />
      ) : null}
      {deleting ? <DeleteDialog workspace={workspace} reply={deleting} onClose={() => setDeleting(null)} /> : null}
    </SettingsPage>
  );
}
