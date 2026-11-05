/*
 * The files of a reply in the composer: a chip for each while it uploads,
 * saying why one failed, with a small preview of a picture, and the
 * button that opens the file picker straight away.
 */
import { RiAttachment2, RiCloseLine, RiErrorWarningLine } from "@remixicon/react";
import { useRef } from "react";

import { SpinnerGlyph } from "@/components/base/spinner/spinner";
import { cx } from "@/utils/cx";

import type { Pending, ReplyFiles } from "./use-reply-files";

function FileChip({ pending, onRemove }: { pending: Pending; onRemove: () => void }) {
  const busy = !pending.attachment && !pending.failure;

  return (
    <li
      className={cx(
        "flex h-9 max-w-56 items-center gap-2 rounded-xl bg-background-secondary-default ps-1.5 pe-1 text-body-2-medium",
        pending.failure ? "text-text-error-primary" : "text-text-primary",
      )}
      title={pending.failure ?? pending.file.name}
    >
      {pending.preview ? (
        <img src={pending.preview} alt="" className="size-6 shrink-0 rounded-md object-cover" />
      ) : (
        <RiAttachment2 aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
      )}
      <span className="min-w-0 flex-1 truncate">{pending.file.name}</span>
      {busy ? <SpinnerGlyph size="sm" track={false} className="text-current" /> : null}
      {pending.failure ? <RiErrorWarningLine aria-hidden className="size-4 shrink-0" /> : null}
      <button
        type="button"
        aria-label={`Remove ${pending.file.name}`}
        onClick={onRemove}
        className="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCloseLine aria-hidden className="size-4" />
      </button>
    </li>
  );
}

/*
 * ReplyFileChips lists the reply's files above the composer's buttons.
 */
export function ReplyFileChips({ files, shown }: { files: ReplyFiles; shown: boolean }) {
  if (!shown || files.files.length === 0) {
    return null;
  }

  return (
    <ul aria-label="Files to send" className="flex flex-wrap gap-2 px-1.5">
      {files.files.map((pending) => (
        <FileChip key={pending.key} pending={pending} onRemove={() => files.remove(pending.key)} />
      ))}
    </ul>
  );
}

/*
 * AttachButton opens the file picker straight away, on a channel that
 * carries files.
 */
export function AttachButton({ onPick, shown }: { onPick: (files: FileList) => void; shown: boolean }) {
  const input = useRef<HTMLInputElement>(null);
  if (!shown) {
    return null;
  }

  return (
    <>
      <input
        ref={input}
        type="file"
        multiple
        hidden
        onChange={(event) => {
          const picked = event.currentTarget.files;
          if (picked && picked.length > 0) {
            onPick(picked);
          }
          event.currentTarget.value = "";
        }}
      />
      <button
        type="button"
        aria-label="Attach files"
        onClick={() => input.current?.click()}
        className="flex size-[30px] shrink-0 cursor-pointer items-center justify-center rounded-full text-foreground-icon-secondary outline-none transition-colors duration-150 ease hover:bg-background-secondary-hover hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiAttachment2 aria-hidden className="size-4" />
      </button>
    </>
  );
}
