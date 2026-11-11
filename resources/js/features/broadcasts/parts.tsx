/*
 * Parts the broadcast pages share: the chips of a state and a channel, the
 * tile of one number, and the preview of a message as the person
 * receiving it will see it: in their inbox, in the messenger or in
 * their Slack channel.
 */
import { RiChat3Line, RiSlackLine } from "@remixicon/react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { KindMark } from "@/features/automation/parts";
import type { BroadcastStatus, Channel } from "@/lib/api/broadcasts";
import { cx } from "@/utils/cx";

import { channelLook, statusLook } from "./looks";
import { MarkdownBody } from "./writer/markdown-body";
import { filled } from "./writer/variables";

export function BroadcastStatusChip({ status }: { status: BroadcastStatus }) {
  const look = statusLook[status];

  return (
    <Chip variant="caption" color={look.tint}>
      {look.name}
    </Chip>
  );
}

export function ChannelMark({ channel, size = "md" }: { channel: Channel; size?: "sm" | "md" }) {
  const look = channelLook[channel];

  return <KindMark icon={look.icon} tint={look.tint} size={size} />;
}

/*
 * PreviewMessage is what a preview shows: where it goes, its subject and
 * preview line, the Markdown body, who it comes from and the name of the
 * team it comes from. A channel without its own look shows as a post.
 */
export interface PreviewMessage {
  channel: string;
  subject: string;
  preview: string;
  body: string;
  sender: string;
  brand?: string;
}

function initials(name: string): string {
  return name
    .split(" ")
    .map((part) => part[0] ?? "")
    .join("")
    .slice(0, 2);
}

function EmailPreview({ broadcast }: { broadcast: PreviewMessage }) {
  return (
    <div className="flex flex-col overflow-hidden rounded-2xl border border-border-button-default bg-background-primary-default">
      <div className="flex flex-col gap-1 border-b border-separator-border px-4 py-3">
        <span className="flex items-center gap-2">
          <Avatar size="xs" initials={initials(broadcast.sender)} />
          <span className="text-body-2-medium text-text-primary">{broadcast.sender}</span>
          <span className="truncate text-body-2-regular text-text-tertiary">from {broadcast.brand ?? "Acme"}</span>
        </span>
        <span className="text-body-medium text-text-primary">{filled(broadcast.subject) || "No subject yet"}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{broadcast.preview}</span>
      </div>
      <div className="flex flex-col gap-3 px-4 py-4">
        <MarkdownBody markdown={broadcast.body} />
        <p className="text-body-regular text-text-secondary">
          {broadcast.sender}, {broadcast.brand ?? "Acme"}
        </p>
      </div>
      <p className="border-t border-separator-border px-4 py-3 text-caption-1-regular text-text-tertiary">
        You get these because you use {broadcast.brand ?? "Acme"}. Unsubscribe.
      </p>
    </div>
  );
}

function ChatPreview({ broadcast }: { broadcast: PreviewMessage }) {
  return (
    <div className="flex flex-col items-end gap-3 rounded-2xl bg-background-tertiary-default p-4">
      <div className="flex w-full max-w-80 flex-col gap-2 rounded-2xl bg-background-primary-default p-4 shadow-card">
        <span className="flex items-center gap-2">
          <Avatar size="xs" initials={initials(broadcast.sender)} />
          <span className="text-body-2-medium text-text-primary">{broadcast.sender}</span>
          <span className="text-caption-1-regular text-text-tertiary">just now</span>
        </span>
        <MarkdownBody markdown={broadcast.body} />
        <span className="mt-1 rounded-xl border border-border-button-default px-3 py-2 text-body-2-regular text-text-placeholder">
          Write a reply…
        </span>
      </div>
      <span
        aria-hidden
        className="flex size-12 items-center justify-center rounded-full bg-accent-500 text-text-white shadow-card"
      >
        <RiChat3Line className="size-6" />
      </span>
    </div>
  );
}

function PostPreview({ broadcast }: { broadcast: PreviewMessage }) {
  return (
    <div className="flex justify-center rounded-2xl bg-background-tertiary-default p-4">
      <div className="flex w-full max-w-96 flex-col gap-3 rounded-2xl bg-background-primary-default p-5 shadow-card">
        <span className="flex items-center gap-2">
          <Avatar size="xs" initials={initials(broadcast.sender)} />
          <span className="text-body-2-regular text-text-secondary">
            {broadcast.sender} from {broadcast.brand ?? "Acme"}
          </span>
        </span>
        <span className="text-title-3-semibold text-text-primary">{broadcast.subject}</span>
        <MarkdownBody markdown={broadcast.body} />
        <span className="self-start rounded-lg bg-accent-500 px-3 py-1.5 text-body-2-medium text-text-white">
          Read the full post
        </span>
      </div>
    </div>
  );
}

function SlackPreview({ broadcast }: { broadcast: PreviewMessage }) {
  const brand = broadcast.brand ?? "Acme";

  return (
    <div className="flex flex-col overflow-hidden rounded-2xl border border-border-button-default bg-background-primary-default">
      <div className="flex items-center gap-2 border-b border-separator-border px-4 py-2.5">
        <RiSlackLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
        <span className="truncate text-body-2-medium text-text-primary"># {brand.toLowerCase()}-northwind</span>
      </div>
      <div className="flex gap-3 px-4 py-4">
        <Avatar size="sm" initials={initials(broadcast.sender)} />
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <span className="flex items-center gap-2">
            <span className="text-body-2-medium text-text-primary">{broadcast.sender}</span>
            <span className="text-caption-1-regular text-text-tertiary">just now</span>
          </span>
          {broadcast.subject ? (
            <span className="text-body-medium text-text-primary">{filled(broadcast.subject)}</span>
          ) : null}
          <MarkdownBody markdown={broadcast.body} />
        </div>
      </div>
    </div>
  );
}

/*
 * MessagePreview shows the broadcast the way its channel delivers it.
 */
export function MessagePreview({ broadcast, className }: { broadcast: PreviewMessage; className?: string }) {
  const previews: Record<string, typeof PostPreview> = {
    email: EmailPreview,
    chat: ChatPreview,
    post: PostPreview,
    slack: SlackPreview,
  };
  const Preview = previews[broadcast.channel] ?? PostPreview;

  return (
    <div className={cx("flex flex-col", className)}>
      <Preview broadcast={broadcast} />
    </div>
  );
}
