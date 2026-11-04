/*
 * The list of an inbox view: one row per conversation with the channel,
 * the customer, who is viewing it, the subject, how long the customer
 * has waited, its priority and who holds it, in the view's order. Rows
 * follow BoardUI's chat list: the open conversation sits on the tertiary
 * surface, the rest light up on hover. A conversation with something the
 * reader has not seen carries a dot and reads in the stronger weight.
 */
import { Link } from "@tanstack/react-router";

import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { avatarColor } from "@/features/shell/avatar-color";
import type { Conversation } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { ago, customerName, subjectOf } from "./conversations";
import { PriorityChip } from "./priority/priority-menu";
import { AssigneeAvatar, ChannelIcon, Viewers, Waiting } from "./row-extras";

export function ConversationList({
  view,
  conversations,
  openId,
  me,
}: {
  view: string;
  conversations: Conversation[];
  openId?: string;
  me?: string;
}) {
  return (
    <ul className="flex flex-col gap-1 px-[7px] pb-[7px]">
      {conversations.map((conversation) => {
        const name = customerName(conversation.contact);
        const open = conversation.id === openId;

        return (
          <li key={conversation.id}>
            <Link
              to="/d/inbox/$view/$conversation"
              params={{ view, conversation: conversation.id }}
              aria-current={open ? "page" : undefined}
              className={cx(
                "flex items-center gap-2.5 rounded-xl px-2 py-1.5 outline-none transition-colors duration-150",
                "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                open ? "bg-background-tertiary-default" : "hover:bg-background-secondary-hover",
              )}
            >
              <Avatar size="md" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2">
                  <ChannelIcon channel={conversation.channel} />
                  <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{name}</span>
                  <Viewers viewers={conversation.viewers} me={me} />
                  <span className="shrink-0 text-caption-1-medium text-text-tertiary">
                    {ago(conversation.last_message_at ?? conversation.created_at)}
                  </span>
                </span>
                <span className="flex items-center gap-2">
                  <span
                    className={cx(
                      "min-w-0 flex-1 truncate text-body-2-regular",
                      conversation.unread ? "text-text-primary" : "text-text-tertiary",
                    )}
                  >
                    {subjectOf(conversation)}
                  </span>
                  <Waiting conversation={conversation} />
                  <PriorityChip priority={conversation.priority} />
                  <AssigneeAvatar assignee={conversation.assignee} />
                  {conversation.unread ? (
                    <>
                      <StatusDot color="indigo" />
                      <span className="sr-only">Unread</span>
                    </>
                  ) : null}
                </span>
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
