/*
 * The conversations a request came up in, newest link first: who asked,
 * from which company, and the conversation itself, which opens in the
 * inbox. Teammates link them from the conversation's details.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { type LinkedConversation, linkedConversationsQuery } from "@/lib/api/feedback";

function who(row: LinkedConversation): string {
  return row.name ?? row.email ?? "Customer";
}

function day(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

/*
 * LinkedConversations lists the conversations linked to the request.
 */
export function LinkedConversations({ workspace, postId }: { workspace: string; postId: string }) {
  const { data, isPending } = useQuery(linkedConversationsQuery(workspace, postId));
  const rows = data?.items ?? [];

  if (isPending) {
    return null;
  }

  if (rows.length === 0) {
    return (
      <p className="text-body-2-regular text-text-tertiary">
        No conversations linked yet. Link one from a conversation's details in the inbox.
      </p>
    );
  }

  return (
    <ul aria-label="Linked conversations" className="flex flex-col divide-y divide-separator-border">
      {rows.map((row) => (
        <li key={row.conversation_id} className="flex items-center gap-3 py-2.5">
          <span className="min-w-0 flex-1">
            <span className="block truncate text-body-2-medium text-text-primary">
              {who(row)}
              {row.company ? <span className="text-text-tertiary"> · {row.company}</span> : null}
            </span>
            <Link
              to="/d/inbox/$view/$conversation"
              params={{ view: "all", conversation: row.conversation_id }}
              className="block truncate text-body-2-regular text-text-secondary outline-none hover:text-text-primary hover:underline focus-visible:underline"
            >
              #{row.conversation_number} {row.subject ?? "Conversation"}
            </Link>
          </span>
          <time dateTime={row.linked_at} className="shrink-0 text-caption-1-medium text-text-tertiary">
            {day(row.linked_at)}
          </time>
        </li>
      ))}
    </ul>
  );
}
