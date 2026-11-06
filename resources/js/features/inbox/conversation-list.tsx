/*
 * The list of an inbox view: one row per conversation with the channel,
 * the customer, who is viewing it, the subject, how long the customer
 * has waited, whether the reader left a draft there, its priority and
 * who holds it, the agent's part in it, and the inbox when the reader
 * sees several, in the view's order. Rows
 * follow BoardUI's chat list: the open conversation sits on the tertiary
 * surface, the rest light up on hover. A conversation with something the
 * reader has not seen carries a dot and reads in the stronger weight.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { avatarColor } from "@/features/shell/avatar-color";
import { type Conversation, inboxesQuery } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { AiBadge } from "./ai-badge";
import { ago, customerName, subjectOf, useInbox } from "./conversations";
import { PriorityChip } from "./priority/priority-menu";
import { AssigneeAvatar, ChannelBadge, DraftMark, InboxLabel, Viewers, Waiting } from "./row-extras";

export function ConversationList({
  view,
  conversations,
  openId,
  me,
  drafted,
}: {
  view: string;
  conversations: Conversation[];
  openId?: string;
  me?: string;
  drafted?: ReadonlySet<string>;
}) {
  const { workspace } = useInbox();
  const { data: inboxes = [] } = useQuery({ ...inboxesQuery(workspace), enabled: workspace !== "" });

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
                "flex items-center gap-3 rounded-xl px-2.5 py-2.5 outline-none transition-colors duration-150",
                "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                open ? "bg-background-tertiary-default" : "hover:bg-background-secondary-hover",
              )}
            >
              <span className="relative shrink-0">
                <Avatar size="lg" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />
                <ChannelBadge channel={conversation.channel} />
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="flex items-center gap-2">
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
                  <DraftMark drafted={drafted?.has(conversation.id) ?? false} />
                  <Waiting conversation={conversation} />
                  <AiBadge state={conversation.ai_state} />
                  <PriorityChip priority={conversation.priority} />
                  <AssigneeAvatar assignee={conversation.assignee} />
                  {conversation.unread ? (
                    <>
                      <StatusDot color="indigo" />
                      <span className="sr-only">Unread</span>
                    </>
                  ) : null}
                </span>
                <InboxLabel inbox={conversation.inbox} inboxes={inboxes} />
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
