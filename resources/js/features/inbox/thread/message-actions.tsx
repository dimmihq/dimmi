/*
 * What a teammate can do to one message from the thread: copy a link to
 * it, edit their own note in place, and delete what they wrote or, when
 * they may moderate, anything a colleague or the customer wrote. Deleting
 * asks first and says what stays: a reply the customer received is still
 * theirs. A note also takes an emoji reaction. The actions show on hover
 * and on keyboard focus.
 */
import { RiDeleteBinLine, RiEditLine, RiLinkM } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { cx } from "@/utils/cx";

import { ReactionPicker } from "./reactions";

const action =
  "flex size-7 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring";

export function MessageActions({
  kind,
  customer,
  canEdit,
  canDelete,
  onCopy,
  onEdit,
  onDelete,
  onReact,
}: {
  kind: "note" | "reply" | "customer";
  customer: string;
  canEdit: boolean;
  canDelete: boolean;
  onCopy: () => void;
  onEdit: () => void;
  onDelete: () => Promise<unknown>;
  onReact?: (emoji: string) => void;
}) {
  const [confirming, setConfirming] = useState(false);
  const description =
    kind === "reply"
      ? `Deleted for your team. ${customer} already received this reply and keeps their copy.`
      : "Deleted for your team. This cannot be undone.";

  return (
    <div
      role="toolbar"
      aria-label="Message actions"
      className={cx(
        "flex items-center gap-0.5 rounded-xl bg-background-primary-default p-0.5 shadow-card",
        "opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-within:opacity-100",
      )}
    >
      {onReact ? <ReactionPicker className={action} onPick={onReact} /> : null}
      <button type="button" aria-label="Copy link" onClick={onCopy} className={action}>
        <RiLinkM aria-hidden className="size-4" />
      </button>
      {canEdit ? (
        <button type="button" aria-label="Edit note" onClick={onEdit} className={action}>
          <RiEditLine aria-hidden className="size-4" />
        </button>
      ) : null}
      {canDelete ? (
        <button type="button" aria-label="Delete message" onClick={() => setConfirming(true)} className={action}>
          <RiDeleteBinLine aria-hidden className="size-4" />
        </button>
      ) : null}
      <AlertDialog
        isOpen={confirming}
        onOpenChange={setConfirming}
        title="Delete this message?"
        description={description}
        confirmLabel="Delete"
        pendingLabel="Deleting…"
        onConfirm={onDelete}
      />
    </div>
  );
}

/*
 * NoteEditor rewrites a note in place: Cmd+Enter or Save keeps it, Esc or
 * Cancel leaves it as it was.
 */
export function NoteEditor({
  text,
  onSave,
  onCancel,
}: {
  text: string;
  onSave: (body: string) => Promise<unknown>;
  onCancel: () => void;
}) {
  const [body, setBody] = useState(text);
  const [saving, setSaving] = useState(false);
  const save = async () => {
    if (!body.trim() || saving) {
      return;
    }
    setSaving(true);
    try {
      await onSave(body.trim());
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex w-full flex-col gap-2">
      <textarea
        aria-label="Edit note"
        ref={(field) => field?.focus()}
        value={body}
        rows={3}
        onChange={(event) => setBody(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.preventDefault();
            onCancel();
          } else if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            void save();
          }
        }}
        className="block w-full resize-none bg-transparent p-0 text-body-regular text-text-primary outline-none"
      />
      <div className="flex justify-end gap-2">
        <Button size="xs" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button size="xs" variant="primary" onClick={() => void save()} disabled={saving || !body.trim()}>
          Save
        </Button>
      </div>
    </div>
  );
}
