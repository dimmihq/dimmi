/*
 * The list of an inbox view: one row per conversation with the customer,
 * the subject and the time of the latest message, newest first. Rows
 * follow BoardUI's chat list: the open conversation sits on the tertiary
 * surface, the rest light up on hover.
 */
import { Link } from "@tanstack/react-router";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { avatarColor } from "@/features/shell/avatar-color";
import { cx } from "@/utils/cx";

import type { Conversation } from "./conversations";

export function ConversationList({
  view,
  conversations,
  openId,
}: {
  view: string;
  conversations: Conversation[];
  openId?: string;
}) {
  return (
    <ul className="flex flex-col gap-1 px-[7px] pb-[7px]">
      {conversations.map((conversation) => (
        <li key={conversation.id}>
          <Link
            to="/d/inbox/$view/$conversation"
            params={{ view, conversation: conversation.id }}
            aria-current={conversation.id === openId ? "page" : undefined}
            className={cx(
              "flex items-center gap-2.5 rounded-xl px-2 py-1.5 outline-none transition-colors duration-150",
              "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              conversation.id === openId ? "bg-background-tertiary-default" : "hover:bg-background-secondary-hover",
            )}
          >
            <Avatar
              size="md"
              color={avatarColor(conversation.customer.name)}
              initials={conversation.customer.initials.charAt(0)}
            />
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="flex items-center gap-2">
                <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">
                  {conversation.customer.name}
                </span>
                <span className="shrink-0 text-caption-1-medium text-text-tertiary">{conversation.updated}</span>
              </span>
              <span className="flex items-center gap-2">
                <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-tertiary">
                  {conversation.subject}
                </span>
                {conversation.example ? (
                  <Chip variant="caption" color={conversation.id === openId ? "soft" : "neutral"} className="py-0">
                    Example
                  </Chip>
                ) : null}
              </span>
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
