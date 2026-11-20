/*
 * What the team knows about a customer or a company, as separate notes. A box at the
 * top adds one; Cmd or Ctrl+Enter sends it and the box clears for the
 * next. Each note shows who wrote it and when. One note can be pinned,
 * so it stays first whatever comes after. A teammate rewrites or deletes
 * their own notes from the note's menu; the first few show, with the
 * rest behind "Show all".
 */
import {
  RiArrowDownSLine,
  RiArrowUpSLine,
  RiDeleteBinLine,
  RiPencilLine,
  RiPushpin2Fill,
  RiPushpin2Line,
  RiUnpinLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type KeyboardEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { NoteComposer } from "@/features/inbox/composer";
import { ago } from "@/features/inbox/conversations";
import { MessageBody } from "@/features/inbox/message-body";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import {
  addContactNote,
  type ContactNote,
  contactNoteKeys,
  contactNotesQuery,
  deleteContactNote,
  type NoteSubject,
  updateContactNote,
} from "@/lib/api/customers";

/*
 * shownFirst is how many notes show before "Show all".
 */
const shownFirst = 3;

type Act = "pin" | "unpin" | "edit" | "delete";

function actionsOf(note: ContactNote): RowAction<Act>[] {
  return [
    note.pinned
      ? { kind: "unpin", label: "Unpin", icon: RiUnpinLine }
      : { kind: "pin", label: "Pin to top", icon: RiPushpin2Line },
    ...(note.editable
      ? ([
          { kind: "edit", label: "Edit", icon: RiPencilLine },
          { kind: "delete", label: "Delete", icon: RiDeleteBinLine, danger: true },
        ] satisfies RowAction<Act>[])
      : []),
  ];
}

/*
 * sendOnEnter runs send on Cmd or Ctrl+Enter.
 */
function sendOnEnter(event: KeyboardEvent, send: () => void) {
  if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
    event.preventDefault();
    send();
  }
}

/*
 * NoteField is the box a note is written in, to add one or to rewrite
 * one: its words, and the buttons that send or drop them.
 */
function NoteField({
  label,
  placeholder,
  start = "",
  action,
  busy,
  onSend,
  onCancel,
}: {
  label: string;
  placeholder?: string;
  start?: string;
  action: string;
  busy: boolean;
  onSend: (body: string) => Promise<unknown>;
  onCancel?: () => void;
}) {
  const [body, setBody] = useState(start);
  const ready = body.trim() !== "" && body.trim() !== start.trim();
  const send = () => {
    if (ready && !busy) {
      void onSend(body.trim()).then(() => setBody(onCancel ? body : ""));
    }
  };

  return (
    <div className="flex flex-col gap-1.5">
      <Textarea
        aria-label={label}
        placeholder={placeholder}
        rows={onCancel ? 2 : 1}
        autoResize
        maxRows={8}
        maxLength={10000}
        autoFocus={onCancel !== undefined}
        value={body}
        onChange={setBody}
        onKeyDown={(event: KeyboardEvent) => sendOnEnter(event, send)}
        className="text-body-2-regular"
      />
      {body.trim() !== "" || onCancel ? (
        <div className="flex items-center justify-end gap-1.5">
          {onCancel ? (
            <Button size="xs" variant="secondary" onClick={onCancel}>
              Cancel
            </Button>
          ) : null}
          <Button size="xs" loading={busy} disabled={!ready} onClick={send}>
            {action}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function NoteRow({
  note,
  busy,
  onAct,
  onRewrite,
}: {
  note: ContactNote;
  busy: boolean;
  onAct: (act: Act) => void;
  onRewrite: (body: string) => Promise<unknown>;
}) {
  const [editing, setEditing] = useState(false);
  if (editing) {
    return (
      <li>
        <NoteField
          label="Edit note"
          start={note.body}
          action="Save"
          busy={busy}
          onSend={(body) => onRewrite(body).then(() => setEditing(false))}
          onCancel={() => setEditing(false)}
        />
      </li>
    );
  }

  return (
    <li className="group/note flex flex-col gap-0.5 py-1.5">
      <MessageBody text={note.body} className="break-words text-body-2-regular text-text-primary" />
      <div className="flex min-h-6 items-center gap-1.5 text-caption-1-regular text-text-tertiary">
        {note.pinned ? (
          <RiPushpin2Fill aria-label="Pinned" className="size-3.5 shrink-0 text-foreground-icon-secondary" />
        ) : null}
        <span className="min-w-0 truncate">
          {note.author?.name ?? "The team"} · {ago(note.created_at)}
          {note.updated_at !== note.created_at ? " · edited" : ""}
        </span>
        <span className="ms-auto opacity-0 transition-opacity duration-150 group-focus-within/note:opacity-100 group-hover/note:opacity-100 has-[[aria-expanded=true]]:opacity-100 pointer-coarse:opacity-100">
          <RowMenu
            quiet
            label={`Note by ${note.author?.name ?? "the team"}`}
            actions={actionsOf(note)}
            onPick={(act) => (act === "edit" ? setEditing(true) : onAct(act))}
          />
        </span>
      </div>
    </li>
  );
}

export function CustomerNotes({
  workspace,
  about,
  composer = false,
}: {
  workspace: string;
  about: NoteSubject;
  /* Writes new notes in the composer conversations and tickets use, where the page has room for it. */
  composer?: boolean;
}) {
  const client = useQueryClient();
  const { data: notes, isPending } = useQuery(contactNotesQuery(workspace, about));
  const [all, setAll] = useState(false);
  const [deleting, setDeleting] = useState<ContactNote | null>(null);
  const refresh = () => client.invalidateQueries({ queryKey: contactNoteKeys.list(workspace, about) });
  const failed = (error: Error) => toast.error(error.message);
  const add = useMutation({
    mutationFn: (body: string) => addContactNote(workspace, about, body),
    onSuccess: refresh,
    onError: failed,
  });
  const change = useMutation({
    mutationFn: ({ id, ...rest }: { id: string; body?: string; pinned?: boolean }) =>
      updateContactNote(workspace, id, rest),
    onSuccess: refresh,
    onError: failed,
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteContactNote(workspace, id),
    onSuccess: refresh,
    onError: failed,
  });
  const list = notes ?? [];
  const hidden = list.length - shownFirst;

  return (
    <div className="flex flex-col gap-1.5">
      {composer ? (
        <NoteComposer onAdd={(body) => add.mutateAsync(body)} />
      ) : (
        <NoteField
          label="New note"
          placeholder={about.kind === "company" ? "Add a note about this company" : "Add a note about this customer"}
          action="Add note"
          busy={add.isPending}
          onSend={(body) => add.mutateAsync(body)}
        />
      )}
      {isPending ? <Skeleton shape="text" className="mt-1 h-3.5 w-40" /> : null}
      <ul
        aria-label={about.kind === "company" ? "Company notes" : "Customer notes"}
        className="flex flex-col divide-y divide-separator-border"
      >
        {(all ? list : list.slice(0, shownFirst)).map((note) => (
          <NoteRow
            key={note.id}
            note={note}
            busy={change.isPending}
            onRewrite={(body) => change.mutateAsync({ id: note.id, body })}
            onAct={(act) => {
              if (act === "delete") {
                setDeleting(note);
              } else {
                change.mutate({ id: note.id, pinned: act === "pin" });
              }
            }}
          />
        ))}
      </ul>
      {hidden > 0 ? (
        <LinkButton
          variant="secondary"
          size="xs"
          className="self-start"
          leadingIcon={all ? RiArrowUpSLine : RiArrowDownSLine}
          aria-expanded={all}
          onClick={() => setAll((was) => !was)}
        >
          {all ? "Show less" : `Show all ${list.length}`}
        </LinkButton>
      ) : null}
      <AlertDialog
        isOpen={deleting !== null}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
        title="Delete this note?"
        description="It is removed for the whole team and cannot be brought back."
        confirmLabel="Delete note"
        onConfirm={() => (deleting ? remove.mutateAsync(deleting.id) : undefined)}
      />
    </div>
  );
}
