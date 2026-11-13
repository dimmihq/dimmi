/*
 * Back-office work shown inside the conversation of the customer request
 * it serves, in that request's ticket panel. Folded, a card names the
 * work, its state and who holds it; opened, it lets a teammate move its
 * state, hand it to someone, read the notes the team left on it and
 * add one, without leaving the conversation.
 */
import { RiArrowDownSLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { ago } from "@/features/inbox/conversations";
import { useAccount } from "@/features/settings/use-account";
import { type LinkedTicket, type Ticket, ticketEventsQuery, ticketQuery } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { StatePill } from "./state-pill";
import { ClashPrompt, StatePicker } from "./ticket-fields";
import { Holders, Note, NoteBox } from "./ticket-thread";
import { useTicketSave } from "./use-ticket-save";
import { useTicketSetup } from "./use-ticket-setup";

function Opened({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const save = useTicketSave(workspace, ticket);
  const setup = useTicketSetup(workspace, ticket);
  const { data } = useQuery(ticketEventsQuery(workspace, ticket.id));
  const notes = (data?.items ?? [])
    .filter((e) => e.kind === "note")
    .sort((a, b) => a.created_at.localeCompare(b.created_at));

  return (
    <div className="flex flex-col gap-3 px-2 pt-1 pb-2">
      <ClashPrompt save={save} />
      <StatePicker ticket={ticket} states={setup.states} save={save} compact />
      <dl className="flex flex-col">
        <Holders workspace={workspace} ticket={ticket} save={save} />
      </dl>
      {notes.length > 0 ? (
        <ol aria-label={`Notes on ${ticket.reference}`} className="flex flex-col gap-3">
          {notes.map((note) => (
            <Note key={note.id} event={note} />
          ))}
        </ol>
      ) : (
        <p className="text-body-2-regular text-text-tertiary">No notes yet.</p>
      )}
      <NoteBox workspace={workspace} ticket={ticket} bare />
    </div>
  );
}

export function BackOfficeCard({
  linked,
  holder,
  startOpen = false,
}: {
  linked: LinkedTicket;
  holder: string;
  startOpen?: boolean;
}) {
  const workspace = useAccount().workspace.id;
  const [open, setOpen] = useState(startOpen);
  const { data: ticket } = useQuery({ ...ticketQuery(workspace, linked.id), enabled: open });

  return (
    <div className="flex flex-col rounded-xl border border-border-button-default">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((was) => !was)}
        className="flex flex-col gap-0.5 rounded-xl px-2 py-1.5 text-start outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <span className="flex min-w-0 items-center gap-1.5 text-body-2-medium text-text-primary">
          <span className="shrink-0 text-text-tertiary">{linked.reference}</span>
          <span className="min-w-0 flex-1 truncate">{linked.title}</span>
          <RiArrowDownSLine
            aria-hidden
            className={cx(
              "size-4 shrink-0 text-foreground-icon-tertiary transition-transform duration-150",
              open && "rotate-180",
            )}
          />
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-caption-1-regular text-text-secondary">
          <StatePill state={linked.state} />
          <span className="truncate">
            {linked.type_name} · {holder} · {ago(linked.updated_at)}
          </span>
        </span>
      </button>
      {open && ticket ? <Opened workspace={workspace} ticket={ticket} /> : null}
    </div>
  );
}
