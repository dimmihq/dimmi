/*
 * A tracker: one problem many customers reported, opened in the inbox
 * like any ticket. The thread in the middle holds the AI's summary of
 * what customers report, drafts an engineer wrote to them waiting for a
 * person, the messages already sent to all of them, every report with
 * its customer, company, plan, revenue and ticket state, then the
 * team's notes and history. Beside it, the details: the state, who
 * holds it, its fields, who it reaches and the engineering work behind
 * it. "Message everyone" writes once to all of them.
 */
import { RiSendPlaneLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/base/buttons/button";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { trackerConversationsQuery } from "@/lib/api/ticket-trackers";
import { type Ticket, ticketQuery } from "@/lib/api/tickets";

import { CloseLoopDrafts } from "./issues/close-loop-drafts";
import { MessageEveryone } from "./message-everyone";
import { SentMessages } from "./message-everyone-results";
import { StatePill, TypeIcon } from "./state-pill";
import { NoteBox, TicketDetails, TicketTitle, Timeline } from "./ticket-thread";
import { TrackerConversations } from "./tracker-conversations";
import { ImpactTotals, Reach, Summary, Title } from "./tracker-parts";
import { useTicketSave } from "./use-ticket-save";

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

function Tracker({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { t } = useTranslation("trackers");
  const { view } = useParams({ strict: false }) as { view?: string };
  const { data: linked, isPending } = useQuery(trackerConversationsQuery(workspace, ticket.id));
  const [messaging, setMessaging] = useState<Messaging>({ kind: "closed" });
  const save = useTicketSave(workspace, ticket);
  const rows = linked?.rows ?? [];

  return (
    <div className="flex h-full min-w-0 flex-1 gap-2">
      <Panel label={t("tracker.label")} className="flex h-full min-w-0 flex-1 flex-col">
        <header className="flex h-12 shrink-0 items-center gap-2 ps-3 pe-2.5 pt-[7px]">
          <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
          <span className="shrink-0 text-body-2-medium text-text-tertiary">{ticket.reference}</span>
          <TicketTitle key={ticket.version} ticket={ticket} save={save} />
          <span className="ms-auto flex shrink-0 items-center gap-2">
            <StatePill state={ticket.state} />
            <Button
              size="small"
              leadingIcon={RiSendPlaneLine}
              disabled={rows.length === 0}
              onClick={() => setMessaging({ kind: "new" })}
            >
              {t("message.open")}
            </Button>
          </span>
        </header>
        <ScrollArea className="flex-1">
          <div className="mx-auto flex w-full max-w-[760px] flex-col gap-6 px-4 pt-5">
            {ticket.description ? (
              <p className="whitespace-pre-line text-body-regular text-text-primary">{ticket.description}</p>
            ) : null}
            <Summary ticket={ticket} />
            <CloseLoopDrafts workspace={workspace} ticket={ticket.id} />
            <SentMessages
              workspace={workspace}
              ticket={ticket.id}
              onOpen={(u) => setMessaging({ kind: "sent", update: u.id })}
            />
            <TrackerConversations workspace={workspace} ticket={ticket} rows={rows} loading={isPending} />
          </div>
          <Timeline workspace={workspace} ticket={ticket} view={view ?? "trackers"} intro={false} />
        </ScrollArea>
        <NoteBox workspace={workspace} ticket={ticket} />
      </Panel>
      <Panel label="Details" className="hidden h-full w-[340px] shrink-0 flex-col lg:flex">
        <header className="flex h-12 shrink-0 items-center ps-4 pe-2.5 pt-[7px]">
          <h2 className="text-body-medium text-text-primary">Details</h2>
        </header>
        <ScrollArea>
          <TicketDetails workspace={workspace} ticket={ticket} save={save}>
            <Reach impact={linked?.impact ?? ticket.impact} />
          </TicketDetails>
        </ScrollArea>
      </Panel>
      {messaging.kind !== "closed" ? (
        <MessageEveryone
          workspace={workspace}
          ticket={ticket}
          rows={rows}
          sentUpdate={messaging.kind === "sent" ? messaging.update : undefined}
          onClose={() => setMessaging({ kind: "closed" })}
        />
      ) : null}
    </div>
  );
}
