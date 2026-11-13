/*
 * A tracker: one problem many customers reported, opened in the inbox
 * like any ticket. The thread in the middle starts with what the tracker
 * is about, then runs oldest first through the team's notes and every
 * change, the messages already sent to everyone (each opening on how it
 * went) and the drafts an engineer's "@customers" comment left for a
 * person to send, with the note box at the foot. Beside it, the
 * details: the state, who holds it, the customer reports with "Reply
 * all", and the engineering work behind it.
 */
import { RiMailSendLine, RiReplyAllLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { ago } from "@/features/inbox/conversations";
import { ConversationSkeleton } from "@/features/inbox/skeletons";
import { useAccount } from "@/features/settings/use-account";
import { Panel, ScrollArea } from "@/features/shell/page";
import { type TicketUpdateDraft, ticketDraftsQuery } from "@/lib/api/ticket-issues";
import {
  type TicketUpdate,
  ticketUpdatesQuery,
  trackerConversationsQuery,
  type UpdateStatus,
} from "@/lib/api/ticket-trackers";
import { type Ticket, type TicketEvent, ticketEventsQuery, ticketQuery } from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";

import { DraftCard } from "./issues/close-loop-drafts";
import { MessageEveryone } from "./message-everyone";
import { StatePill, TypeIcon } from "./state-pill";
import { Change, foldLinks, LinkRun, Note, NoteBox, TicketDetails, TicketTitle } from "./ticket-thread";
import { CustomerReports } from "./tracker-conversations";
import { useTicketSave } from "./use-ticket-save";

/*
 * Messaging is the message dialog's state: closed, writing a new one,
 * or showing one already sent.
 */
type Messaging = { kind: "closed" } | { kind: "new" } | { kind: "sent"; update: string };

/*
 * Entry is one item of the thread, in the order it happened.
 */
type Entry =
  | { kind: "event"; at: string; event: TicketEvent }
  | { kind: "sent"; at: string; update: TicketUpdate }
  | { kind: "draft"; at: string; draft: TicketUpdateDraft };

const updateTint: Record<UpdateStatus, ChipColor> = {
  queued: "gray",
  sending: "blue",
  sent: "lime",
  partial: "yellow",
  failed: "rose",
};

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
 * Sent is a message already sent to everyone, a compact line in the
 * thread that opens on how each delivery went.
 */
function Sent({ update, onOpen }: { update: TicketUpdate; onOpen: () => void }) {
  const { t } = useTranslation("trackers");
  const counts = t("results.counts", {
    sent: formatNumber(update.delivered),
    held: formatNumber(update.held),
    failed: formatNumber(update.failed),
    total: formatNumber(update.recipients),
  });

  return (
    <li>
      <button
        type="button"
        onClick={onOpen}
        title={counts}
        className="flex w-full min-w-0 items-center gap-2 rounded-lg px-1 py-1 text-start text-caption-1-regular text-text-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiMailSendLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="shrink-0 text-body-2-medium text-text-secondary">{t("results.entry")}</span>
        <span className="min-w-0 flex-1 truncate">{update.body.split("\n")[0]}</span>
        <Chip variant="caption" color={updateTint[update.status]} className="shrink-0">
          {t(`results.status.${update.status}`)}
        </Chip>
        <span className="shrink-0">{ago(update.created_at)}</span>
      </button>
    </li>
  );
}

/*
 * Thread is the middle of the page: the description, then notes,
 * changes, messages sent and drafts, oldest first.
 */
function Thread({
  workspace,
  ticket,
  onOpenSent,
}: {
  workspace: string;
  ticket: Ticket;
  onOpenSent: (update: TicketUpdate) => void;
}) {
  const { data: events } = useQuery(ticketEventsQuery(workspace, ticket.id));
  const { data: updates = [] } = useQuery(ticketUpdatesQuery(workspace, ticket.id));
  const { data: drafts = [] } = useQuery(ticketDraftsQuery(workspace, ticket.id));
  const entries: Entry[] = [
    ...(events?.items ?? []).map((event): Entry => ({ kind: "event", at: event.created_at, event })),
    ...updates.map((update): Entry => ({ kind: "sent", at: update.created_at, update })),
    ...drafts
      .filter((d) => d.status === "ready")
      .map((draft): Entry => ({ kind: "draft", at: draft.created_at, draft })),
  ].sort((a, b) => a.at.localeCompare(b.at));

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-4 px-4 py-5">
      {ticket.description ? (
        <p className="whitespace-pre-wrap text-body-regular text-text-primary">{ticket.description}</p>
      ) : null}
      <ol aria-label="Notes and history" className="flex flex-col gap-3">
        {foldLinks(entries, (entry) =>
          entry.kind === "event" && entry.event.kind === "link" ? entry.event : null,
        ).map((entry) => {
          if (Array.isArray(entry)) {
            return <LinkRun key={entry[0]?.id} events={entry} />;
          }
          if (entry.kind === "sent") {
            return <Sent key={entry.update.id} update={entry.update} onOpen={() => onOpenSent(entry.update)} />;
          }
          if (entry.kind === "draft") {
            return <DraftCard key={entry.draft.id} workspace={workspace} ticket={ticket.id} draft={entry.draft} />;
          }

          return entry.event.kind === "note" ? (
            <Note key={entry.event.id} event={entry.event} />
          ) : (
            <Change key={entry.event.id} event={entry.event} />
          );
        })}
      </ol>
    </div>
  );
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
              variant="secondary"
              leadingIcon={RiReplyAllLine}
              disabled={rows.length === 0}
              onClick={() => setMessaging({ kind: "new" })}
              className="lg:hidden"
            >
              {t("conversations.replyAll")}
            </Button>
          </span>
        </header>
        <ScrollArea className="flex-1">
          <Thread
            workspace={workspace}
            ticket={ticket}
            onOpenSent={(u) => setMessaging({ kind: "sent", update: u.id })}
          />
        </ScrollArea>
        <NoteBox workspace={workspace} ticket={ticket} />
      </Panel>
      <Panel label="Details" className="hidden h-full w-[340px] shrink-0 flex-col lg:flex">
        <header className="flex h-12 shrink-0 items-center ps-4 pe-2.5 pt-[7px]">
          <h2 className="text-body-medium text-text-primary">Details</h2>
        </header>
        <ScrollArea>
          <TicketDetails workspace={workspace} ticket={ticket} save={save}>
            <CustomerReports
              workspace={workspace}
              view={view ?? "trackers"}
              ticket={ticket}
              rows={rows}
              impact={linked?.impact ?? ticket.impact ?? undefined}
              loading={isPending}
              onReplyAll={() => setMessaging({ kind: "new" })}
            />
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
