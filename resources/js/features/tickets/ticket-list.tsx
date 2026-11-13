/*
 * A ticket view of the inbox: the open tickets of a type, of every type,
 * waiting on another team, or the open trackers, newest change first.
 * Each row shows the reference, the type's icon, the title, the state as
 * a pill in its group's colour, who holds it and how many conversations
 * it links. A customer ticket opens its conversation beside the list,
 * since its replies live there; back-office work and trackers open their
 * own ticket page. Search and filters narrow the view; a page of fifty
 * loads at a time.
 */
import { RiCoupon3Line, RiLinksLine, RiSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useDeferredValue, useEffect, useMemo, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { ago, useInbox } from "@/features/inbox/conversations";
import { ConversationRowsSkeleton } from "@/features/inbox/skeletons";
import { rememberView } from "@/features/inbox/views/last-view";
import { ViewPicker } from "@/features/inbox/views/view-picker";
import { avatarColor } from "@/features/shell/avatar-color";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { requestPage } from "@/lib/api/client";
import { type Ticket, ticketParams, ticketsQuery, ticketTypesQuery } from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { StatePill, TypeIcon } from "./state-pill";
import { type Filters, TicketFilters } from "./ticket-filters";
import { ticketViewCount, ticketViewFilter, ticketViewName } from "./views";

const pageSize = 50;

/*
 * useMore reads the pages after the first, starting over whenever the
 * filter changes.
 */
function useMore(workspace: string, query: string, first: { next: string | null } | undefined) {
  const [state, setState] = useState<{ query: string; items: Ticket[]; next: string | null; busy: boolean }>({
    query,
    items: [],
    next: null,
    busy: false,
  });
  const current = state.query === query ? state : { query, items: [], next: null, busy: false };
  const cursor = current.items.length > 0 ? current.next : (first?.next ?? null);
  const load = async () => {
    if (!cursor) {
      return;
    }
    setState({ ...current, busy: true });
    const params = new URLSearchParams(query);
    params.set("cursor", cursor);
    const page = await requestPage<Ticket>(`/tickets?${params.toString()}`, { workspace });
    setState({ query, items: [...current.items, ...page.items], next: page.next, busy: false });
  };

  return { items: current.items, more: cursor !== null, busy: current.busy, load };
}

export function TicketListPanel({ view, openId }: { view: string; openId: string | undefined }) {
  const { workspace } = useInbox();
  const { data: types } = useQuery(ticketTypesQuery(workspace));
  const title = ticketViewName(view, types) ?? "Tickets";
  const [search, setSearch] = useState("");
  const q = useDeferredValue(search.trim());
  const [filters, setFilters] = useState<Filters>({});
  const filter = { ...ticketViewFilter(view), ...filters, q, limit: pageSize };
  const { data: first, isPending, isError } = useQuery(ticketsQuery(workspace, filter));
  const more = useMore(workspace, ticketParams(filter), first);
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const names = useMemo(() => new Map(members.map((m) => [m.id, m.user.name])), [members]);
  const rows = [...(first?.items ?? []), ...more.items];
  useEffect(() => rememberView(view), [view]);

  return (
    <Panel label={`${title} tickets`} className="h-full w-full">
      <PageHeader
        title={<ViewPicker workspace={workspace} view={view} title={title} count={ticketViewCount(view, types)} />}
      />
      <div className="shrink-0 px-[7px] pb-2">
        <Input
          aria-label="Search tickets"
          placeholder="Search tickets or T-482"
          leadingIcon={RiSearchLine}
          value={search}
          onChange={setSearch}
        />
      </div>
      <TicketFilters
        workspace={workspace}
        filters={filters}
        viewTypeId={view.startsWith("tktt_") ? view : undefined}
        onChange={setFilters}
      />
      <ListBody
        state={isPending ? "loading" : isError ? "error" : "ready"}
        view={view}
        title={title}
        q={q}
        rows={rows}
        openId={openId}
        names={names}
        more={more}
      />
    </Panel>
  );
}

function ListBody({
  state,
  view,
  title,
  q,
  rows,
  openId,
  names,
  more,
}: {
  state: "loading" | "error" | "ready";
  view: string;
  title: string;
  q: string;
  rows: Ticket[];
  openId: string | undefined;
  names: Map<string, string>;
  more: ReturnType<typeof useMore>;
}) {
  if (state === "loading") {
    return <ConversationRowsSkeleton />;
  }
  if (state === "error") {
    return (
      <EmptyState
        icon={RiCoupon3Line}
        title="Tickets did not load"
        description="Check your connection and try again."
      />
    );
  }
  if (rows.length === 0) {
    return (
      <EmptyState
        icon={RiCoupon3Line}
        title="No open tickets"
        description={q ? `Nothing in this view matches “${q}”.` : "Tickets in this view show here."}
      />
    );
  }

  return (
    <ScrollArea>
      <ul aria-label={title} className="flex flex-col gap-1 px-[7px] pb-[7px]">
        {rows.map((ticket) => (
          <TicketRow
            key={ticket.id}
            view={view}
            ticket={ticket}
            assignee={ticket.assignee_id ? names.get(ticket.assignee_id) : undefined}
            open={openId !== undefined && (openId === ticket.conversation_id || openId === ticket.id)}
          />
        ))}
      </ul>
      {more.more ? (
        <div className="flex justify-center px-[7px] pt-1 pb-3">
          <Button size="small" variant="secondary" disabled={more.busy} onClick={() => void more.load()}>
            Load more
          </Button>
        </div>
      ) : null}
    </ScrollArea>
  );
}

const rowClass = "flex flex-col gap-1 rounded-xl px-2.5 py-2.5 outline-none transition-colors duration-150";

function TicketRow({
  view,
  ticket,
  assignee,
  open,
}: {
  view: string;
  ticket: Ticket;
  assignee: string | undefined;
  open: boolean;
}) {
  const body = (
    <>
      <span className="flex items-center gap-2">
        <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
        <span className="shrink-0 text-caption-1-medium text-text-tertiary">{ticket.reference}</span>
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{ticket.title}</span>
        <span className="shrink-0 text-caption-1-medium text-text-tertiary">{ago(ticket.updated_at)}</span>
      </span>
      <span className="flex items-center gap-2 ps-6">
        <StatePill state={ticket.state} />
        <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-tertiary">{ticket.type.name}</span>
        {ticket.linked_conversations > 0 ? (
          <span
            className="flex shrink-0 items-center gap-0.5 text-caption-1-medium text-text-secondary"
            title={`${ticket.linked_conversations} linked conversations`}
          >
            <RiLinksLine aria-hidden className="size-3.5" />
            {ticket.linked_conversations}
          </span>
        ) : null}
        {assignee ? (
          <Avatar size="xs" color={avatarColor(assignee)} initials={assignee.charAt(0)} aria-label={assignee} />
        ) : null}
      </span>
    </>
  );
  const tone = open ? "bg-background-tertiary-default" : "hover:bg-background-secondary-hover";
  const focus = "focus-visible:ring-2 focus-visible:ring-border-focus-ring";

  return (
    <li>
      <Link
        to="/d/inbox/$view/$conversation"
        params={{
          view,
          conversation: ticket.kind === "customer" && ticket.conversation_id ? ticket.conversation_id : ticket.id,
        }}
        aria-current={open ? "page" : undefined}
        className={cx(rowClass, tone, focus)}
      >
        {body}
      </Link>
    </li>
  );
}
