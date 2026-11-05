/*
 * What a list row shows beside the customer and subject: a badge on the
 * avatar for the channel the conversation came through, who holds it, how long the customer has
 * waited for an answer, and which teammates have it open right now, up
 * to three faces and then a count. The reader never sees themselves.
 */
import {
  RiChat3Line,
  RiDraftLine,
  RiGlobalLine,
  RiMailLine,
  RiPlugLine,
  RiTimeLine,
  RiWhatsappLine,
} from "@remixicon/react";

import { Focusable } from "react-aria-components";

import { Avatar } from "@/components/base/avatar/avatar";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { avatarColor } from "@/features/shell/avatar-color";
import type { Conversation, Viewer } from "@/lib/api/inbox";

import { waitedFor } from "./conversations";

const channels: Record<string, { icon: typeof RiMailLine; label: string }> = {
  email: { icon: RiMailLine, label: "Email" },
  widget: { icon: RiChat3Line, label: "Messenger" },
  api: { icon: RiPlugLine, label: "API" },
  whatsapp: { icon: RiWhatsappLine, label: "WhatsApp" },
};

/*
 * ChannelBadge marks the corner of a customer's avatar with the channel
 * the conversation came through, named in a tooltip on hover.
 */
export function ChannelBadge({ channel }: { channel: Conversation["channel"] }) {
  const known = channels[channel.type];
  const Icon = known?.icon ?? RiGlobalLine;
  const label = channel.name || known?.label || channel.type;

  return (
    <TooltipTrigger>
      <Focusable>
        <span
          role="img"
          aria-label={`Via ${label}`}
          className="absolute -end-1 -bottom-1 grid size-5 place-items-center rounded-full bg-background-primary-default text-foreground-icon-secondary shadow-xs ring-1 ring-border-button-default"
        >
          <Icon aria-hidden className="block size-3" />
        </span>
      </Focusable>
      <Tooltip>Via {label}</Tooltip>
    </TooltipTrigger>
  );
}

export function AssigneeAvatar({ assignee }: { assignee: Conversation["assignee"] }) {
  if (!assignee) {
    return null;
  }

  return (
    <Avatar
      size="xs"
      color={avatarColor(assignee.name)}
      initials={assignee.name.charAt(0).toUpperCase()}
      aria-label={`Assigned to ${assignee.name}`}
      role="img"
    />
  );
}

/*
 * Waiting says how long the customer has waited, on open conversations
 * only.
 */
export function Waiting({ conversation }: { conversation: Conversation }) {
  if (conversation.status !== "open" || !conversation.waiting_since) {
    return null;
  }

  return (
    <span className="flex shrink-0 items-center gap-0.5 text-caption-1-medium text-text-tertiary">
      <RiTimeLine aria-hidden className="size-3" />
      <span>Waiting {waitedFor(conversation.waiting_since)}</span>
    </span>
  );
}

/*
 * DraftMark says the reader left a reply or note unsent here.
 */
export function DraftMark({ drafted }: { drafted: boolean }) {
  if (!drafted) {
    return null;
  }

  return (
    <span className="flex shrink-0 items-center gap-0.5 text-caption-1-medium text-text-secondary">
      <RiDraftLine aria-hidden className="size-3" />
      <span>Draft</span>
    </span>
  );
}

export function Viewers({ viewers, me }: { viewers: Viewer[] | undefined; me: string | undefined }) {
  const others = (viewers ?? []).filter((v) => v.id !== me);
  if (others.length === 0) {
    return null;
  }
  const names = others.map((v) => v.name).join(", ");

  return (
    <span role="img" aria-label={`Viewing: ${names}`} title={names} className="flex shrink-0 items-center -space-x-1">
      {others.slice(0, 3).map((v) => (
        <Avatar
          key={v.id}
          size="xs"
          color={avatarColor(v.name)}
          initials={v.name.charAt(0).toUpperCase()}
          src={v.avatar_url ?? undefined}
          className="ring-1 ring-background-primary-default"
        />
      ))}
      {others.length > 3 ? (
        <span className="ps-1.5 text-caption-1-medium text-text-tertiary">+{others.length - 3}</span>
      ) : null}
    </span>
  );
}
