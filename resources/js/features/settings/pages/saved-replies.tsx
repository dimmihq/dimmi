/*
 * Saved replies and macros: the answers the team sends often, shared or
 * one person's own, with variables filled in for each customer and the
 * actions a macro applies when it is sent. Anyone keeps their own;
 * sharing, and changing what is shared, show only to people the server
 * lets manage shared replies. A CSV of title, body and shared adds many
 * at once, all or none.
 */
import { RiDeleteBinLine, RiPencilLine, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useRef, useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { readReplies } from "@/features/saved-replies/csv";
import { hasActions, macroSummary, type Names } from "@/features/saved-replies/macro";
import { ReplyDialog } from "@/features/saved-replies/reply-dialog";
import { useNames } from "@/features/saved-replies/use-names";
import {
  deleteSavedReply,
  type ImportedReply,
  importSavedReplies,
  type SavedReply,
  savedRepliesKey,
  savedRepliesQuery,
} from "@/lib/api/saved-replies";
import { callerQuery } from "@/lib/api/workspace";
import { Action } from "../controls";
import { SettingsPage } from "../layout";
import { placeholders } from "../placeholders";
import { failureText } from "../teams";
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
    <li className="flex items-start gap-3 rounded-2xl bg-background-secondary-default px-3 py-2.5">
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-body-medium text-text-primary">{reply.title}</span>
          {reply.shortcut ? (
            <span className="shrink-0 text-body-2-regular text-text-tertiary">#{reply.shortcut}</span>
          ) : null}
          <Badge className="shrink-0">{reply.shared ? "Shared" : "Only me"}</Badge>
        </span>
        <span className="line-clamp-2 text-body-2-regular text-text-secondary">{reply.body}</span>
        {summary.length > 0 ? (
          <span className="truncate text-body-2-medium text-text-tertiary">Then: {summary.join(" · ")}</span>
        ) : null}
      </span>
      {canChange ? (
        <span className="flex shrink-0 items-center gap-2">
          <Button
            iconOnly
            variant="secondary"
            size="small"
            leadingIcon={RiPencilLine}
            aria-label={`Edit ${reply.title}`}
            onClick={onEdit}
          />
          <Button
            iconOnly
            variant="danger-secondary"
            size="small"
            leadingIcon={RiDeleteBinLine}
            aria-label={`Delete ${reply.title}`}
            onClick={onDelete}
          />
        </span>
      ) : null}
    </li>
  );
}

function RowsSkeleton() {
  return placeholders(4).map((id) => (
    <li key={id} aria-hidden className="flex h-[68px] flex-col gap-2 rounded-2xl bg-background-secondary-default p-3">
      <Skeleton shape="text" className="h-3 w-40" />
      <Skeleton shape="text" className="h-3 w-64" />
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

/*
 * ImportDialog confirms the replies a CSV holds before adding them.
 */
function ImportDialog({
  workspace,
  replies,
  canShare,
  onClose,
}: {
  workspace: string;
  replies: ImportedReply[];
  canShare: boolean;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const rows = canShare ? replies : replies.map((reply) => ({ ...reply, shared: false }));
  const shared = rows.filter((reply) => reply.shared).length;
  const run = useMutation({
    mutationFn: () => importSavedReplies(workspace, rows),
    onSuccess: () => client.invalidateQueries({ queryKey: savedRepliesKey(workspace) }),
  });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      variant="default"
      icon={false}
      title={`Import ${rows.length} saved ${rows.length === 1 ? "reply" : "replies"}?`}
      description={`${shared} shared with the team, ${rows.length - shared} only yours.`}
      confirmLabel="Import"
      onConfirm={() => run.mutateAsync()}
    >
      <p className="line-clamp-6 text-body-2-regular text-text-secondary">
        {rows
          .slice(0, 50)
          .map((reply) => reply.title)
          .join(", ")}
      </p>
      {run.error ? (
        <p role="alert" className="mt-2 text-body-2-regular text-text-error-primary">
          {failureText(run.error)}
        </p>
      ) : null}
    </AlertDialog>
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
  const [importing, setImporting] = useState<ImportedReply[] | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const file = useRef<HTMLInputElement>(null);

  return (
    <SettingsPage
      title="Saved replies"
      description="Answers your team sends often. Insert them from the composer, or type # and a shortcut."
    >
      <section aria-label="Saved replies" className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2 px-3">
          <Input
            aria-label="Search saved replies"
            placeholder="Search saved replies"
            leadingIcon={RiSearchLine}
            value={search}
            onChange={setSearch}
            className="min-w-48 flex-1"
          />
          <Action onPress={() => file.current?.click()}>Import CSV</Action>
          <Action onPress={() => setEditing("new")}>New saved reply</Action>
          <input
            ref={file}
            type="file"
            accept=".csv,text/csv"
            aria-label="CSV file of saved replies"
            className="hidden"
            onChange={async (event) => {
              const picked = event.target.files?.[0];
              event.target.value = "";
              if (!picked) {
                return;
              }
              const read = readReplies(await picked.text());
              setFileError(read.error);
              setImporting(read.error ? null : read.replies);
            }}
          />
        </div>
        <p className="px-3 text-body-2-regular text-text-tertiary">
          A CSV needs title and body columns; shortcut and shared (yes or no) are optional.
        </p>
        {fileError ? (
          <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
            {fileError}
          </p>
        ) : null}
        {replies?.length === 0 ? (
          <p className="rounded-2xl bg-background-secondary-default px-3 py-2.5 text-body-2-regular text-text-tertiary">
            {search.trim() ? "No saved replies match." : "No saved replies yet."}
          </p>
        ) : null}
        <ul aria-busy={!replies || undefined} className="flex flex-col gap-2">
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
      {importing ? (
        <ImportDialog
          workspace={workspace}
          replies={importing}
          canShare={canShare}
          onClose={() => setImporting(null)}
        />
      ) : null}
    </SettingsPage>
  );
}
