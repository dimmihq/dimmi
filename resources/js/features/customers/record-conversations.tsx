/*
 * The conversations of a customer or a company, newest first, each
 * opening in the inbox's view of every conversation.
 */
import { Link } from "@tanstack/react-router";

import { StatusDot } from "@/components/base/badges/status-dot";
import { statusLabel } from "@/features/inbox/case";
import { ago, subjectOf } from "@/features/inbox/conversations";
import { Nothing } from "@/features/inbox/customer/rows";
import { ListSkeleton } from "@/features/shell/table-skeleton";
import type { Conversation } from "@/lib/api/inbox";

const statusDot = { open: "green", snoozed: "yellow", closed: "indigo" } as const;

export function RecordConversations({
  conversations,
  loading = false,
}: {
  conversations: Conversation[];
  loading?: boolean;
}) {
  if (loading) {
    return <ListSkeleton />;
  }

  if (conversations.length === 0) {
    return <Nothing>No conversations yet.</Nothing>;
  }

  return (
    <ul className="-mx-2 flex flex-col">
      {conversations.map((conversation) => (
        <li key={conversation.id}>
          <Link
            to="/d/inbox/$view/$conversation"
            params={{ view: "all", conversation: conversation.id }}
            className="flex min-h-9 items-center gap-2 rounded-xl px-2 text-body-2-regular outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <span className="shrink-0 text-text-tertiary">#{conversation.number}</span>
            <span className="min-w-0 flex-1 truncate text-text-primary">{subjectOf(conversation)}</span>
            <StatusDot color={statusDot[conversation.status]} />
            <span className="shrink-0 text-text-secondary">{statusLabel[conversation.status]}</span>
            <span className="shrink-0 text-caption-1-medium text-text-tertiary">
              {ago(conversation.last_message_at ?? conversation.created_at)}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
