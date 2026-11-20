/*
 * The customer's other conversations, newest first: the three the
 * context carries, and up to twenty once the teammate asks for more.
 * Each opens in place, in the view the teammate is working.
 */
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";
import { useState } from "react";
import { StatusDot } from "@/components/base/badges/status-dot";
import { LinkButton } from "@/components/base/buttons/link-button";
import { type ConversationContext, contactConversationsQuery } from "@/lib/api/customers";
import type { Conversation } from "@/lib/api/inbox";

import { statusLabel } from "../case";
import { ago, subjectOf } from "../conversations";
import { Section } from "./rows";

const statusDot = { open: "green", snoozed: "yellow", closed: "indigo" } as const;
const most = 20;

function Item({ conversation, view }: { conversation: Conversation; view: string }) {
  return (
    <li>
      <Link
        to="/d/inbox/$view/$conversation"
        params={{ view, conversation: conversation.id }}
        className="flex min-h-8 items-center gap-2 rounded-lg px-1.5 text-body-2-regular outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <StatusDot color={statusDot[conversation.status]} />
        <span className="sr-only">{statusLabel[conversation.status]}</span>
        <span className="shrink-0 text-text-tertiary tabular-nums">#{conversation.number}</span>
        <span className="min-w-0 flex-1 truncate text-text-primary">{subjectOf(conversation)}</span>
        <span className="shrink-0 text-caption-1-medium text-text-tertiary">
          {ago(conversation.last_message_at ?? conversation.created_at)}
        </span>
      </Link>
    </li>
  );
}

export function PastConversations({
  workspace,
  context,
  current,
}: {
  workspace: string;
  context: ConversationContext;
  current: string;
}) {
  const { view } = useParams({ strict: false });
  const [more, setMore] = useState(false);
  const { data: all } = useQuery({
    ...contactConversationsQuery(workspace, context.contact.id, most + 1),
    enabled: more,
  });
  const past = context.past_conversations;
  const shown = more && all ? all.filter((c) => c.id !== current).slice(0, most) : past.data;

  if (past.data.length === 0) {
    return null;
  }

  return (
    <Section title="Past conversations" count={past.total_estimate}>
      <ul className="-mx-1.5 flex flex-col">
        {shown.map((conversation) => (
          <Item key={conversation.id} conversation={conversation} view={view ?? "all"} />
        ))}
      </ul>
      {!more && past.total_estimate > past.data.length ? (
        <div>
          <LinkButton variant="secondary" size="xs" onClick={() => setMore(true)}>
            Show more
          </LinkButton>
        </div>
      ) : null}
    </Section>
  );
}
