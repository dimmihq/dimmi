/*
 * A tracker: one problem many customers reported, opened in the inbox
 * like any ticket. The thread in the middle starts with what the tracker
 * is about, then runs oldest first through the team's notes and every
 * change, with the note box at the foot. Beside it, the details: the
 * state, who holds it, the customer reports, and the engineering work
 * behind it.
 */
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { ConversationSkeleton } from "@/features/inbox/skeletons";
import { useAccount } from "@/features/settings/use-account";
import { Panel, ScrollArea } from "@/features/shell/page";
import { trackerConversationsQuery } from "@/lib/api/ticket-trackers";
import { type Ticket, ticketEventsQuery, ticketQuery } from "@/lib/api/tickets";

import { foldLinks } from "./fold-links";
import { StatePill, TypeIcon } from "./state-pill";
import {
  Change,
  DetailsColumn,
  LinkRun,
  Note,
  NoteBox,
  TicketDetails,
  TicketSplit,
  TicketTitle,
} from "./ticket-thread";
import { CustomerReports } from "./tracker-conversations";
import { useTicketSave } from "./use-ticket-save";

export function TrackerPage({ ticketId }: { ticketId: string }) {
  const { t } = useTranslation("trackers");
  const workspace = useAccount().workspace.id;
  const { data: ticket } = useQuery(ticketQuery(workspace, ticketId));

  if (!ticket) {
    return (
      <Panel label={t("tracker.label")} className="flex-1">
        <ConversationSkeleton />
      </Panel>
    );
  }

  return <Tracker workspace={workspace} ticket={ticket} />;
}

/*
 * Thread is the middle of the page: the description, then notes and
 * changes, oldest first.
 */
function Thread({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { data } = useQuery(ticketEventsQuery(workspace, ticket.id));
  const events = [...(data?.items ?? [])].sort((a, b) => a.created_at.localeCompare(b.created_at));

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-4 px-4 py-5">
      {ticket.description ? (
        <p className="whitespace-pre-wrap text-body-regular text-text-primary">{ticket.description}</p>
      ) : null}
      <ol aria-label="Notes and history" className="flex flex-col gap-3">
        {foldLinks(events, (event) => (event.kind === "link" ? event : null)).map((item) => {
          if (Array.isArray(item)) {
            return <LinkRun key={item[0]?.id} events={item} />;
          }

          return item.kind === "note" ? <Note key={item.id} event={item} /> : <Change key={item.id} event={item} />;
        })}
      </ol>
    </div>
  );
}

function Tracker({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const { t } = useTranslation("trackers");
  const { view } = useParams({ strict: false }) as { view?: string };
  const { data: linked, isPending } = useQuery(trackerConversationsQuery(workspace, ticket.id));
  const save = useTicketSave(workspace, ticket);

  return (
    <TicketSplit
      thread={
        <Panel label={t("tracker.label")} className="flex h-full min-w-0 flex-1 flex-col">
          <header className="flex h-12 shrink-0 items-center gap-2 ps-3 pe-2.5 pt-[7px]">
            <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
            <span className="shrink-0 text-body-2-medium text-text-tertiary">{ticket.reference}</span>
            <TicketTitle key={ticket.version} ticket={ticket} save={save} />
            <span className="ms-auto flex shrink-0 items-center gap-2">
              <StatePill state={ticket.state} />
            </span>
          </header>
          <ScrollArea className="flex-1">
            <Thread workspace={workspace} ticket={ticket} />
          </ScrollArea>
          <NoteBox workspace={workspace} ticket={ticket} />
        </Panel>
      }
      details={
        <DetailsColumn>
          <TicketDetails workspace={workspace} ticket={ticket} save={save} open={[t("conversations.title")]}>
            <CustomerReports
              workspace={workspace}
              view={view ?? "trackers"}
              ticket={ticket}
              rows={linked?.rows ?? []}
              impact={linked?.impact ?? ticket.impact ?? undefined}
              loading={isPending}
            />
          </TicketDetails>
        </DetailsColumn>
      }
    />
  );
}
