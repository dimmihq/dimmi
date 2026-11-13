/*
 * The ticket panel beside a conversation that is a ticket: its
 * reference, its type with a way to change it, the state and what the
 * customer sees of it, its fields with the AI's suggestions, the
 * back-office work linked to it, each a card that opens to move its
 * state, hand it over and read and add the team's notes,
 * the trackers the conversation joined, the issues behind it and when
 * the next reply is due. A conversation that is no ticket but joined
 * trackers shows only those.
 */
import { RiAlarmLine, RiCoupon3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { conversationTicketsQuery, type SLAClock, type Ticket, type TicketType } from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { BackOfficeCard } from "./back-office-card";
import { TicketIssues } from "./issues/ticket-issues";
import { dueIn } from "./labels";
import { StatePill, TypeIcon } from "./state-pill";
import { ClashPrompt, FieldsEditor, StatePicker } from "./ticket-fields";
import { type TicketSave, useTicketSave } from "./use-ticket-save";
import { useTicketSetup } from "./use-ticket-setup";

function Block({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5 border-t border-separator-border pt-3">
      <span className="text-caption-1-medium text-text-tertiary">{label}</span>
      {children}
    </div>
  );
}

function TypeRow({ ticket, types, save }: { ticket: Ticket; types: TicketType[]; save: TicketSave }) {
  const same = types.filter((t) => t.kind === ticket.kind && !t.archived_at);

  return (
    <div className="flex items-center gap-2">
      <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
      <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{ticket.type.name}</span>
      {same.length > 1 ? (
        <Select
          aria-label="Change type"
          size="sm"
          className="w-36"
          selectedKey={ticket.type_id}
          renderValue={() => "Change type"}
          isDisabled={save.busy}
          onSelectionChange={(key) => {
            if (key !== null && key !== ticket.type_id) {
              void save.save({ type_id: String(key) });
            }
          }}
        >
          {same.map((t) => (
            <SelectItem key={t.id} id={t.id} textValue={t.name}>
              {t.name}
            </SelectItem>
          ))}
        </Select>
      ) : null}
    </div>
  );
}

/*
 * nextDue is the running clock that falls due first.
 */
function nextDue(clocks: SLAClock[] | undefined): SLAClock | undefined {
  return (clocks ?? [])
    .filter((c) => !c.stopped_at && !c.paused_at)
    .sort((a, b) => a.due_at.localeCompare(b.due_at))[0];
}

const targetNames: Record<string, string> = {
  first_reply: "First reply",
  next_reply: "Next reply",
  follow_up: "Follow-up",
  resolution: "Resolution",
};

function SlaLine({ clocks }: { clocks: SLAClock[] | undefined }) {
  const clock = nextDue(clocks);
  if (!clock) {
    return <span className="text-body-2-regular text-text-tertiary">No service level on this ticket.</span>;
  }
  const due = dueIn(clock.due_at);

  return (
    <span
      className={cx(
        "flex items-center gap-1.5 text-body-2-medium",
        due.late ? "text-text-error-primary" : "text-text-primary",
      )}
    >
      <RiAlarmLine aria-hidden className="size-4 shrink-0" />
      {targetNames[clock.target] ?? clock.target} {due.late ? `overdue by ${due.text}` : `due in ${due.text}`}
    </span>
  );
}

function LinkedRow({
  ticket,
  detail,
}: {
  ticket: { id: string; reference: string; title: string };
  detail: React.ReactNode;
}) {
  const { view } = useParams({ strict: false }) as { view?: string };

  return (
    <Link
      to="/d/inbox/$view/$conversation"
      params={{ view: view ?? "tickets", conversation: ticket.id }}
      className="flex flex-col gap-0.5 rounded-lg px-1.5 py-1 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <span className="flex min-w-0 items-center gap-1.5 text-body-2-medium text-text-primary">
        <span className="shrink-0 text-text-tertiary">{ticket.reference}</span>
        <span className="truncate">{ticket.title}</span>
      </span>
      <span className="flex min-w-0 items-center gap-1.5 text-caption-1-regular text-text-secondary">{detail}</span>
    </Link>
  );
}

function OwnTicket({ workspace, ticket, trackers }: { workspace: string; ticket: Ticket; trackers: Ticket[] }) {
  const save = useTicketSave(workspace, ticket);
  const setup = useTicketSetup(workspace, ticket);
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const nameOf = (id: string | null) => members.find((m) => m.id === id)?.user.name ?? "Unassigned";
  const backOffice = (ticket.links ?? []).filter((l) => l.kind === "back_office" && l.ticket);

  return (
    <div className="flex flex-col gap-3">
      <ClashPrompt save={save} />
      <TypeRow ticket={ticket} types={setup.types} save={save} />
      <StatePicker ticket={ticket} states={setup.states} save={save} />
      {ticket.close_loop ? (
        <span className="text-body-2-medium text-text-primary">Close the loop: the work behind it is done.</span>
      ) : null}
      <FieldsEditor key={ticket.version} workspace={workspace} ticket={ticket} type={setup.type} save={save} />
      <Block label="Back-office">
        {backOffice.length === 0 ? (
          <span className="text-body-2-regular text-text-tertiary">No back-office work yet.</span>
        ) : (
          backOffice.map((l) =>
            l.ticket ? (
              <BackOfficeCard
                key={l.id}
                linked={l.ticket}
                holder={nameOf(l.ticket.assignee_id)}
                startOpen={backOffice.length === 1}
              />
            ) : null,
          )
        )}
      </Block>
      <TrackersBlock trackers={trackers} />
      <Block label="Issues">
        <TicketIssues ticketId={ticket.id} compact />
      </Block>
      <Block label="SLA">
        <SlaLine clocks={ticket.sla_clocks} />
      </Block>
    </div>
  );
}

function TrackersBlock({ trackers }: { trackers: Ticket[] }) {
  return (
    <Block label="Trackers">
      {trackers.length === 0 ? (
        <span className="text-body-2-regular text-text-tertiary">Not on a tracker.</span>
      ) : (
        trackers.map((t) => (
          <LinkedRow
            key={t.id}
            ticket={t}
            detail={
              <>
                <StatePill state={t.state} />
                <span className="truncate">
                  {t.linked_conversations} {t.linked_conversations === 1 ? "customer" : "customers"}
                </span>
              </>
            }
          />
        ))
      )}
    </Block>
  );
}

export function TicketPanel({
  workspace,
  conversation,
  onConvert,
}: {
  workspace: string;
  conversation: string;
  onConvert: () => void;
}) {
  const { data } = useQuery(conversationTicketsQuery(workspace, conversation));
  if (!data) {
    return null;
  }

  return (
    <section aria-label="Ticket" className="flex flex-col gap-3 border-t border-separator-border pt-3 pb-4">
      <header className="flex items-center gap-2">
        <RiCoupon3Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        <h3 className="flex-1 text-body-2-medium text-text-primary">Ticket</h3>
        {data.ticket ? (
          <span className="text-body-2-medium text-text-secondary">{data.ticket.reference}</span>
        ) : (
          <Button size="xs" variant="secondary" onClick={onConvert}>
            Convert to ticket
          </Button>
        )}
      </header>
      {data.ticket ? (
        <OwnTicket workspace={workspace} ticket={data.ticket} trackers={data.trackers} />
      ) : data.trackers.length > 0 ? (
        <TrackersBlock trackers={data.trackers} />
      ) : (
        <p className="text-body-2-regular text-text-tertiary">Not a ticket. Press T to convert it.</p>
      )}
    </section>
  );
}
