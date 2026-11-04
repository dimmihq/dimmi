/*
 * What a list row shows beside the customer and subject: the channel the
 * conversation came through, who holds it, how long the customer has
 * waited for an answer, and which teammates have it open right now, up
 * to three faces and then a count. The reader never sees themselves.
 */
import {
  RiCodeSSlashLine,
  RiGlobalLine,
  RiMailLine,
  RiMessage3Line,
  RiTimeLine,
  RiWhatsappLine,
} from "@remixicon/react";

import { Avatar } from "@/components/base/avatar/avatar";
import { avatarColor } from "@/features/shell/avatar-color";
import type { Conversation, Viewer } from "@/lib/api/inbox";

import { waitedFor } from "./conversations";

const channelIcons: Record<string, typeof RiMailLine> = {
  email: RiMailLine,
  widget: RiMessage3Line,
  api: RiCodeSSlashLine,
  whatsapp: RiWhatsappLine,
};

export function ChannelIcon({ channel }: { channel: Conversation["channel"] }) {
  const Icon = channelIcons[channel.type] ?? RiGlobalLine;

  return <Icon aria-label={channel.name || channel.type} className="size-3.5 shrink-0 text-foreground-icon-tertiary" />;
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
