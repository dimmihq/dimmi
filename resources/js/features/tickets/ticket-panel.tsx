/*
 * The ticket part of the details beside a conversation that is a
 * ticket: its reference, then rows for its type, its state, what the
 * customer sees of it, which of the customer's companies it is about
 * when they have several, and when the next reply is due, each changed by
 * pressing its value. Below them its fields with the AI's suggestions,
 * and, only when there are any, the back-office work linked to it, the
 * trackers the conversation joined and the issues behind it. A
 * conversation that is no ticket but joined trackers shows only those,
 * and one that is neither offers to become a ticket.
 */
import { RiAddLine, RiAlarmLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Select, SelectItem } from "@/components/base/select/select";
import { Row, Section } from "@/features/inbox/customer/rows";
import { conversationTicketsQuery, type SLAClock, type Ticket, type TicketType } from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { BackOfficeCard } from "./back-office-card";
import { TicketIssues } from "./issues/ticket-issues";
import { dueIn } from "./labels";
import { StatePill, TypeIcon } from "./state-pill";
import { TicketCompanyRow } from "./ticket-company";
import { ClashPrompt, FieldsEditor, StateMenu } from "./ticket-fields";
import { type TicketSave, useTicketSave } from "./use-ticket-save";
import { useTicketSetup } from "./use-ticket-setup";

function TypeValue({ ticket, types, save }: { ticket: Ticket; types: TicketType[]; save: TicketSave }) {
  const same = types.filter((t) => t.kind === ticket.kind && !t.archived_at);
  const shown = (
    <span className="flex min-w-0 items-center gap-1.5">
      <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
      <span className="truncate">{ticket.type.name}</span>
    </span>
  );
  if (same.length < 2) {
    return shown;
  }

  return (
    <Select
      aria-label="Type"
      size="sm"
      variant="quiet"
      selectedKey={ticket.type_id}
      renderValue={() => shown}
      isDisabled={save.busy}
      onSelectionChange={(key) => {
        if (key !== null && key !== ticket.type_id) {
          void save.save({ type_id: String(key) });
        }
      }}
    >
      {same.map((t) => (
        <SelectItem key={t.id} id={t.id} textValue={t.name}>
          <TypeIcon icon={t.icon} kind={t.kind} />
          {t.name}
        </SelectItem>
      ))}
    </Select>
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
    return null;
  }
  const due = dueIn(clock.due_at);

  return (
    <span
      className={cx("flex min-w-0 items-center gap-1.5", due.late ? "text-text-error-primary" : "text-text-primary")}
    >
      <RiAlarmLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="truncate">
        {targetNames[clock.target] ?? clock.target} {due.late ? `overdue by ${due.text}` : `due in ${due.text}`}
      </span>
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
      className="-mx-1.5 flex flex-col gap-0.5 rounded-lg px-1.5 py-1 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <span className="flex min-w-0 items-center gap-1.5 text-body-2-medium text-text-primary">
        <span className="shrink-0 text-text-tertiary">{ticket.reference}</span>
        <span className="truncate">{ticket.title}</span>
      </span>
      <span className="flex min-w-0 items-center gap-1.5 text-caption-1-regular text-text-secondary">{detail}</span>
    </Link>
  );
}

function OwnTicket({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const save = useTicketSave(workspace, ticket);
  const setup = useTicketSetup(workspace, ticket);
  const current = setup.states.find((state) => state.id === ticket.state_id);

  return (
    <div className="flex flex-col gap-1.5">
      <ClashPrompt save={save} />
      <dl className="flex flex-col">
        <Row label="Type">
          <TypeValue ticket={ticket} types={setup.types} save={save} />
        </Row>
        <Row label="State">
          <StateMenu ticket={ticket} states={setup.states} save={save} />
        </Row>
        <Row label="Customer sees">
          <span className="truncate">{current?.customer_label ?? ticket.state.customer_label}</span>
        </Row>
        <TicketCompanyRow workspace={workspace} ticket={ticket} save={save} />
        {nextDue(ticket.sla_clocks) ? (
          <Row label="SLA">
            <SlaLine clocks={ticket.sla_clocks} />
          </Row>
        ) : null}
      </dl>
      {ticket.close_loop ? (
        <span className="text-body-2-medium text-text-primary">Close the loop: the work behind it is done.</span>
      ) : null}
      <FieldsEditor key={ticket.version} workspace={workspace} ticket={ticket} type={setup.type} save={save} />
    </div>
  );
}

/*
 * BackOffice lists the back-office work linked to the ticket, when
 * there is any.
 */
function BackOffice({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const nameOf = (id: string | null) => members.find((m) => m.id === id)?.user.name ?? "Unassigned";
  const work = (ticket.links ?? []).flatMap((l) => (l.kind === "back_office" && l.ticket ? [l.ticket] : []));
  if (work.length === 0) {
    return null;
  }

  return (
    <Section title="Back-office" count={work.length}>
      <div className="flex flex-col">
        {work.map((linked) => (
          <BackOfficeCard
            key={linked.id}
            linked={linked}
            holder={nameOf(linked.assignee_id)}
            startOpen={work.length === 1}
          />
        ))}
      </div>
    </Section>
  );
}

function TrackersBlock({ trackers }: { trackers: Ticket[] }) {
  if (trackers.length === 0) {
    return null;
  }

  return (
    <Section title="Trackers" count={trackers.length}>
      <div className="flex flex-col">
        {trackers.map((t) => (
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
        ))}
      </div>
    </Section>
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

  if (!data.ticket && data.trackers.length === 0) {
    return (
      <Section title="Ticket">
        <div className="flex min-h-8 items-center">
          <LinkButton variant="secondary" size="xs" leadingIcon={RiAddLine} onClick={onConvert}>
            Convert to ticket
          </LinkButton>
        </div>
      </Section>
    );
  }

  return (
    <>
      {data.ticket ? (
        <Section title="Ticket" aside={data.ticket.reference}>
          <OwnTicket workspace={workspace} ticket={data.ticket} />
        </Section>
      ) : null}
      {data.ticket ? <BackOffice workspace={workspace} ticket={data.ticket} /> : null}
      <TrackersBlock trackers={data.trackers} />
      {data.ticket ? <TicketIssues ticketId={data.ticket.id} compact heading="Issues" /> : null}
    </>
  );
}
