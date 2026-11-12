/*
 * A tracker: one problem many customers reported. At the top, what it
 * is, the state it is in and the issue behind it with its status; then
 * who it reaches, as customers, companies and revenue at stake; the
 * AI's summary of what they report, once the server writes one; the
 * messages already sent to all of them; and every linked conversation
 * with its customer, company, plan, revenue and ticket state. "Message
 * everyone" writes once to all of them.
 */
import { RiSendPlaneLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { trackerConversationsQuery } from "@/lib/api/ticket-trackers";
import { type Ticket, ticketQuery } from "@/lib/api/tickets";

import { TicketIssues } from "./issues/ticket-issues";
import { MessageEveryone } from "./message-everyone";
import { SentMessages } from "./message-everyone-results";
import { TrackerConversations } from "./tracker-conversations";
import { useChangeTicket, useTypeStates } from "./tracker-helpers";
import { ImpactTotals, IssueChips, Summary, Title } from "./tracker-parts";

/*
 * Messaging is the message dialog's state: closed, writing a new one,
 * or showing one already sent.
 */
type Messaging = { kind: "closed" } | { kind: "new" } | { kind: "sent"; update: string };

export function TrackerPage({ ticketId }: { ticketId: string }) {
  const { t } = useTranslation("trackers");
  const workspace = useAccount().workspace.id;
  const { data: ticket } = useQuery(ticketQuery(workspace, ticketId));

  if (!ticket) {
    return (
      <Panel label={t("tracker.label")} className="h-full w-full flex-1">
        <PageHeader title={<Title ticket={undefined} fallback={t("tracker.label")} />} />
        <div className="mx-auto w-full max-w-[64rem] px-4 pt-6 md:px-6">
          <ImpactTotals impact={undefined} />
        </div>
      </Panel>
    );
  }

  return <Tracker workspace={workspace} ticket={ticket} />;
}

/*
 * Heading is what the tracker is, the state it is in with the words its
 * customers see, and the issue behind it.
 */
function Heading({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { t } = useTranslation("trackers");
  const states = useTypeStates(workspace, ticket);
  const change = useChangeTicket(workspace, ticket);

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-title-2-medium tracking-tight text-text-primary">{ticket.title}</h2>
      {ticket.description ? (
        <p className="whitespace-pre-line text-body-regular text-text-secondary">{ticket.description}</p>
      ) : null}
      <div className="flex flex-wrap items-end gap-3">
        <Select
          label={t("tracker.state")}
          className="w-56"
          selectedKey={ticket.state_id}
          isDisabled={change.isPending || states.length === 0}
          onSelectionChange={(key) =>
            key && key !== ticket.state_id ? change.mutate({ state_id: String(key) }) : undefined
          }
        >
          {states.map((s) => (
            <SelectItem key={s.id} id={s.id} textValue={s.name}>
              {s.name}
            </SelectItem>
          ))}
        </Select>
        <IssueChips issues={ticket.issues ?? []} />
      </div>
      <p className="text-caption-1-regular text-text-tertiary">
        {t("tracker.customerSees", { label: ticket.state.customer_label })}
      </p>
    </div>
  );
}

function Tracker({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { t } = useTranslation("trackers");
  const { data: linked, isPending } = useQuery(trackerConversationsQuery(workspace, ticket.id));
  const [messaging, setMessaging] = useState<Messaging>({ kind: "closed" });
  const rows = linked?.rows ?? [];

  return (
    <Panel label={t("tracker.label")} className="h-full w-full flex-1">
      <PageHeader title={<Title ticket={ticket} fallback={t("tracker.label")} />}>
        <Button
          size="small"
          leadingIcon={RiSendPlaneLine}
          disabled={rows.length === 0}
          onClick={() => setMessaging({ kind: "new" })}
        >
          {t("message.open")}
        </Button>
      </PageHeader>
      <ScrollArea>
        <div className="mx-auto flex w-full max-w-[64rem] flex-col gap-6 px-4 pt-6 pb-16 md:px-6">
          <Heading workspace={workspace} ticket={ticket} />
          <TicketIssues ticketId={ticket.id} />
          <ImpactTotals impact={linked?.impact ?? ticket.impact} />
          <Summary ticket={ticket} />
          <SentMessages
            workspace={workspace}
            ticket={ticket.id}
            onOpen={(u) => setMessaging({ kind: "sent", update: u.id })}
          />
          <TrackerConversations workspace={workspace} ticket={ticket} rows={rows} loading={isPending} />
        </div>
      </ScrollArea>
      {messaging.kind !== "closed" ? (
        <MessageEveryone
          workspace={workspace}
          ticket={ticket}
          rows={rows}
          sentUpdate={messaging.kind === "sent" ? messaging.update : undefined}
          onClose={() => setMessaging({ kind: "closed" })}
        />
      ) : null}
    </Panel>
  );
}
