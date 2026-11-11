/*
 * The writing step of a broadcast, laid out like a changelog entry: the
 * subject as the title, the preview text as the line under it, who it
 * comes from, a rule, then the message in the same writer help center
 * articles and changelog updates use. A chat message has no subject, so
 * it starts at the byline, and a Slack post's headline is optional.
 * Enter in the title moves to the line under
 * it, and from there into the message.
 */
import { type KeyboardEvent, lazy, Suspense, useRef } from "react";
import { TextArea, TextField } from "react-aria-components";

import { Avatar } from "@/components/base/avatar/avatar";
import { EditorSkeleton } from "@/features/shell/table-skeleton";
import type { Channel } from "@/lib/api/broadcasts";
import { cx } from "@/utils/cx";

import type { Draft } from "./draft";

const MessageWriter = lazy(() => import("./writer/message-writer"));

const bareField =
  "w-full resize-none border-0 bg-transparent p-0 outline-none [field-sizing:content] placeholder:text-text-placeholder";

function nextOnEnter(event: KeyboardEvent, next: () => void) {
  if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
    event.preventDefault();
    next();
  }
}

function initials(name: string): string {
  return name
    .split(" ")
    .map((part) => part[0] ?? "")
    .join("")
    .slice(0, 2);
}

function titleOf(channel: Channel): string {
  return { email: "Subject", post: "Title", slack: "Headline", chat: "" }[channel];
}

function placeholderOf(channel: Channel): string {
  return { email: "Subject", post: "Untitled post", slack: "Headline, optional", chat: "" }[channel];
}

/*
 * Written is the part of a message the writer changes.
 */
export type Written = Pick<Draft, "subject" | "preview" | "body">;

export function WriteStep({
  workspace,
  channel,
  draft,
  sender,
  onChange,
}: {
  workspace: string;
  channel: Channel;
  draft: Written;
  sender: string;
  onChange: (change: Partial<Written>) => void;
}) {
  const page = useRef<HTMLElement | null>(null);
  const focus = (selector: string) => page.current?.querySelector<HTMLElement>(selector)?.focus();
  const email = channel === "email";

  return (
    <article ref={page} className="flex w-full flex-col">
      {channel === "chat" ? null : (
        <TextField
          aria-label={titleOf(channel)}
          value={draft.subject}
          onChange={(subject) => onChange({ subject })}
          maxLength={150}
        >
          <TextArea
            rows={1}
            placeholder={placeholderOf(channel)}
            onKeyDown={(event) => nextOnEnter(event, () => focus(email ? "textarea[data-summary]" : ".ProseMirror"))}
            className={cx(bareField, "text-display-4-medium tracking-tight text-balance text-text-primary")}
          />
        </TextField>
      )}
      {email ? (
        <TextField aria-label="Preview text" value={draft.preview} onChange={(preview) => onChange({ preview })}>
          <TextArea
            rows={1}
            data-summary=""
            placeholder="Add the preview text inboxes show after the subject"
            onKeyDown={(event) => nextOnEnter(event, () => focus(".ProseMirror"))}
            className={cx(bareField, "mt-3 text-title-3-regular text-text-secondary")}
          />
        </TextField>
      ) : null}
      <span className="mt-5 inline-flex items-center gap-2 self-start text-body-2-regular text-text-secondary">
        <Avatar size="xs" initials={initials(sender)} />
        From {sender}
      </span>
      <div className="mt-5 mb-8 border-b border-separator-border" />
      <Suspense fallback={<EditorSkeleton />}>
        <MessageWriter
          workspace={workspace}
          value={draft.body}
          label="Message"
          onChange={(body) => onChange({ body })}
        />
      </Suspense>
    </article>
  );
}
