/*
 * A changelog entry as it will read, laid out like the changelog's entry
 * page: the cover across the top, the labels in their colors, the title,
 * the summary as a subtitle, who wrote it, a rule, then the body in the
 * same writer help center articles use. The cover is picked, changed or
 * removed right on it. Enter in the title moves to the summary, and from
 * there into the body.
 */
import { RiAddLine, RiCheckLine, RiDeleteBinLine, RiImageAddLine, RiImageEditLine } from "@remixicon/react";
import { type CSSProperties, type KeyboardEvent, lazy, Suspense, useRef, useState } from "react";
import { TextArea, TextField } from "react-aria-components";

import { Avatar } from "@/components/base/avatar/avatar";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Notification } from "@/components/base/notification/notification";
import { toast } from "@/components/base/toast/toast";
import { EditorSkeleton } from "@/features/shell/table-skeleton";
import type { ChangelogEntry, ChangelogLabel } from "@/lib/api/changelog";
import { uploadArticleImage } from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { type Draft, initialsOf } from "./entry-format";

const ArticleWriter = lazy(() => import("@/features/help-center/writer/article-writer"));

const bareField =
  "w-full resize-none border-0 bg-transparent p-0 outline-none [field-sizing:content] placeholder:text-text-placeholder";

function nextOnEnter(event: KeyboardEvent, next: () => void) {
  if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
    event.preventDefault();
    next();
  }
}

/*
 * Cover is the entry's picture across the top: a quiet button to add
 * one, or the picture with buttons to change or remove it.
 */
function Cover({ workspace, url, onChange }: { workspace: string; url: string; onChange: (url: string) => void }) {
  const input = useRef<HTMLInputElement | null>(null);
  const [busy, setBusy] = useState(false);
  const pick = async (file: File | undefined) => {
    if (!file) {
      return;
    }
    setBusy(true);
    try {
      onChange(await uploadArticleImage(workspace, file));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The cover was not uploaded.");
    } finally {
      setBusy(false);
    }
  };
  const button = cx(
    "inline-flex h-8 items-center gap-1.5 rounded-lg bg-background-primary-default/90 px-2.5 text-body-2-medium",
    "text-text-primary shadow-xs outline-none backdrop-blur-sm hover:bg-background-primary-default",
    "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
  );

  return (
    <>
      <input
        ref={input}
        type="file"
        hidden
        accept="image/png,image/jpeg,image/gif,image/webp"
        aria-label="Upload a cover"
        onChange={(event) => {
          void pick(event.target.files?.[0]);
          event.target.value = "";
        }}
      />
      {url ? (
        <figure className="group relative mb-8 overflow-hidden rounded-2xl border border-separator-border">
          <img src={url} alt="" className={cx("aspect-video w-full object-cover", busy && "opacity-60")} />
          <span className="absolute end-3 top-3 flex gap-2 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 pointer-coarse:opacity-100">
            <button type="button" className={button} onClick={() => input.current?.click()}>
              <RiImageEditLine aria-hidden className="size-4" />
              Change cover
            </button>
            <button type="button" className={button} onClick={() => onChange("")} aria-label="Remove cover">
              <RiDeleteBinLine aria-hidden className="size-4" />
            </button>
          </span>
        </figure>
      ) : (
        <button
          type="button"
          onClick={() => input.current?.click()}
          disabled={busy}
          className={cx(
            "mb-6 inline-flex h-8 items-center gap-1.5 self-start rounded-lg px-2 text-body-2-medium text-text-tertiary",
            "outline-none transition-colors hover:bg-background-secondary-hover hover:text-text-primary",
            "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          )}
        >
          <RiImageAddLine aria-hidden className="size-4" />
          {busy ? "Uploading cover…" : "Add cover"}
        </button>
      )}
    </>
  );
}

/*
 * Labels shows the entry's labels as colored chips and a menu to file
 * it under others.
 */
function Labels({
  labels,
  chosen,
  onChange,
}: {
  labels: ChangelogLabel[];
  chosen: string[];
  onChange: (labels: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const toggle = (id: string) => onChange(chosen.includes(id) ? chosen.filter((l) => l !== id) : [...chosen, id]);

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {labels
        .filter((l) => chosen.includes(l.id))
        .map((l) => (
          <span
            key={l.id}
            className="inline-flex h-6 items-center gap-1.5 rounded-full bg-background-secondary-default px-2.5 text-caption-1-medium text-text-secondary"
          >
            <span aria-hidden className="size-1.5 rounded-full" style={{ background: l.color }} />
            {l.name}
          </span>
        ))}
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="Labels"
          className="inline-flex h-6 items-center gap-1 rounded-full px-2 text-caption-1-medium text-text-tertiary outline-none hover:bg-background-secondary-hover hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiAddLine aria-hidden className="size-3.5" />
          {chosen.length === 0 ? "Add label" : "Label"}
        </DropdownTrigger>
        <DropdownPopover aria-label="Labels" placement="bottom start" className="w-56 p-1">
          {labels.length === 0 ? (
            <p className="px-2 py-1.5 text-body-2-regular text-text-tertiary">No labels yet.</p>
          ) : null}
          {labels.map((l) => (
            <DropdownItem
              key={l.id}
              selected={chosen.includes(l.id)}
              onSelect={() => toggle(l.id)}
              className="px-2 py-1.5"
            >
              <span aria-hidden className="size-2 rounded-full" style={{ background: l.color }} />
              <span className="flex-1 truncate text-body-medium text-text-primary">{l.name}</span>
              {chosen.includes(l.id) ? (
                <RiCheckLine aria-hidden className="size-4 text-foreground-icon-secondary" />
              ) : null}
            </DropdownItem>
          ))}
        </DropdownPopover>
      </Dropdown>
    </div>
  );
}

/*
 * Byline shows who wrote the entry, and opens the author picker.
 */
function Byline({ entry, onEdit }: { entry: ChangelogEntry; onEdit: () => void }) {
  return (
    <button
      type="button"
      onClick={onEdit}
      aria-label="Authors"
      className="mt-5 inline-flex items-center gap-2 self-start rounded-lg py-1 pe-2 text-body-2-regular text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <span className="flex -space-x-1.5">
        {entry.authors.slice(0, 4).map((a) => (
          <Avatar
            key={a.member_id}
            size="xs"
            src={a.avatar_url ?? undefined}
            initials={initialsOf(a.name)}
            className="ring-2 ring-background-secondary-default"
          />
        ))}
      </span>
      {entry.authors.length > 0 ? entry.authors.map((a) => a.name).join(", ") : "Add authors"}
      {entry.authors_chosen ? null : <span className="text-text-tertiary">· from edits</span>}
    </button>
  );
}

export function EntryPage({
  workspace,
  entry,
  draft,
  labels,
  brand,
  revision,
  failure,
  onChange,
  onAuthors,
}: {
  workspace: string;
  entry: ChangelogEntry;
  draft: Draft;
  labels: ChangelogLabel[];
  brand: string;
  revision: number;
  failure: Error | null;
  onChange: (change: Partial<Draft>) => void;
  onAuthors: () => void;
}) {
  const page = useRef<HTMLElement | null>(null);
  const focus = (selector: string) => page.current?.querySelector<HTMLElement>(selector)?.focus();

  return (
    <article
      ref={page}
      className="article-brand mx-auto flex w-full max-w-[42.5rem] flex-col px-6 pt-10 md:pt-14"
      style={{ "--brand": brand } as CSSProperties}
    >
      {failure ? (
        <Notification status="error" title={failure.message} dismissible={false} role="alert" className="mb-6 w-full" />
      ) : null}
      <Cover workspace={workspace} url={draft.cover} onChange={(cover) => onChange({ cover })} />
      <Labels labels={labels} chosen={draft.labels} onChange={(next) => onChange({ labels: next })} />
      <TextField aria-label="Title" value={draft.title} onChange={(title) => onChange({ title })} maxLength={150}>
        <TextArea
          rows={1}
          placeholder="Untitled update"
          onKeyDown={(event) => nextOnEnter(event, () => focus("textarea[data-summary]"))}
          className={cx(bareField, "mt-3 text-display-4-medium tracking-tight text-balance text-text-primary")}
        />
      </TextField>
      <TextField
        aria-label="Summary"
        value={draft.summary}
        onChange={(summary) => onChange({ summary })}
        maxLength={300}
      >
        <TextArea
          rows={1}
          data-summary=""
          placeholder="Add a one-line summary"
          onKeyDown={(event) => nextOnEnter(event, () => focus(".ProseMirror"))}
          className={cx(bareField, "mt-3 text-title-3-regular text-text-secondary")}
        />
      </TextField>
      <Byline entry={entry} onEdit={onAuthors} />
      <div className="mt-5 mb-8 border-b border-separator-border" />
      <Suspense fallback={<EditorSkeleton />}>
        <ArticleWriter
          key={revision}
          workspace={workspace}
          value={draft.body}
          label="Update"
          brand={brand}
          onChange={(body) => onChange({ body })}
        />
      </Suspense>
    </article>
  );
}
