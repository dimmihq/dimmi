/*
 * The customers who reported what a tracker follows, as a compact list
 * in its details: each linked conversation's customer, their company
 * and the state of their own ticket, opening that conversation in the
 * inbox. One quiet line above says who it reaches in all. Teammates
 * link more conversations from a picker of the inbox's own
 * conversations and unlink one after confirming. "Reply all" opens a
 * draft broadcast that answers every linked customer in their own
 * conversation; it leaves out, until a teammate says otherwise, those
 * still waiting on an answer and those already answered since the
 * tracker last moved.
 */
import {
  RiAddLine,
  RiArrowDownSLine,
  RiArrowUpSLine,
  RiLinkUnlinkM,
  RiReplyAllLine,
  RiSearchLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { StatusDot } from "@/components/base/badges/status-dot";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { toast } from "@/components/base/toast/toast";
import { ago } from "@/features/inbox/conversations";
import { onRowHover } from "@/features/inbox/customer/quiet";
import { Nothing, Section } from "@/features/inbox/customer/rows";
import { createBroadcast } from "@/lib/api/broadcasts";
import { conversationsQuery } from "@/lib/api/inbox";
import { type Impact, type TrackerConversation, trackerKeys } from "@/lib/api/ticket-trackers";
import { addTicketLink, removeTicketLink, type Ticket, ticketKeys } from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";

import { customerName, money } from "./tracker-helpers";

/*
 * shownFirst is how many reports show before "Show all".
 */
const shownFirst = 8;

const statusDot: Record<string, "green" | "yellow" | "indigo"> = { open: "green", snoozed: "yellow", closed: "indigo" };

/*
 * Report is one customer's report: its conversation's number, title
 * and status, opening the conversation; the customer shows on hover, as
 * does unlinking.
 */
function Report({ view, row, onUnlink }: { view: string; row: TrackerConversation; onUnlink: () => void }) {
  const { t } = useTranslation("trackers");

  return (
    <li className="group/row flex items-center gap-1">
      <Link
        to="/d/inbox/$view/$conversation"
        params={{ view, conversation: row.conversation_id }}
        className="flex min-h-8 min-w-0 flex-1 items-center gap-2 rounded-lg px-1.5 text-body-2-regular outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <StatusDot color={statusDot[row.status] ?? "gray"} />
        <span className="sr-only">{row.status}</span>
        <span className="shrink-0 text-text-tertiary tabular-nums">#{row.conversation_number}</span>
        <span className="min-w-0 flex-1 truncate text-text-primary" title={customerName(row)}>
          {row.subject || t("conversations.untitled")}
        </span>
        <span className="shrink-0 text-caption-1-medium text-text-tertiary">
          {ago(row.last_message_at ?? row.linked_at)}
        </span>
      </Link>
      <Button
        variant="secondary"
        size="xs"
        iconOnly
        leadingIcon={RiLinkUnlinkM}
        aria-label={t("conversations.unlink", { number: row.conversation_number })}
        className={onRowHover}
        onClick={onUnlink}
      />
    </li>
  );
}

/*
 * ReplyAll starts a conversation broadcast for the tracker and opens it
 * to be written.
 */
function ReplyAll({ workspace, ticket, disabled }: { workspace: string; ticket: Ticket; disabled: boolean }) {
  const { t } = useTranslation("trackers");
  const navigate = useNavigate();
  const start = useMutation({
    mutationFn: () =>
      createBroadcast(
        workspace,
        t("conversations.replyAllName", { reference: ticket.reference, title: ticket.title }).slice(0, 120),
        "conversation",
        { ticket_id: ticket.id, holds: ["customer_waiting", "team_replied"] },
      ),
    onSuccess: (broadcast) => void navigate({ to: "/d/broadcasts/$broadcast", params: { broadcast: broadcast.id } }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <Button
      size="xs"
      variant="secondary"
      leadingIcon={RiReplyAllLine}
      disabled={disabled}
      loading={start.isPending}
      onClick={() => start.mutate()}
    >
      {t("conversations.replyAll")}
    </Button>
  );
}

/*
 * CustomerReports is the block in the tracker's details: the count, who
 * it reaches, the reports, and the way to link more.
 */
export function CustomerReports({
  workspace,
  view,
  ticket,
  rows,
  impact,
  loading,
}: {
  workspace: string;
  view: string;
  ticket: Ticket;
  rows: TrackerConversation[];
  impact: Impact | undefined;
  loading: boolean;
}) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [all, setAll] = useState(false);
  const [unlinking, setUnlinking] = useState<TrackerConversation | null>(null);
  const refresh = () => {
    void client.invalidateQueries({ queryKey: trackerKeys.conversations(workspace, ticket.id) });
    void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
  };
  const shown = all ? rows : rows.slice(0, shownFirst);

  return (
    <Section title={t("conversations.title")} count={rows.length}>
      <div className="flex flex-col gap-1">
        {impact && impact.customers > 0 ? (
          <p className="text-body-2-regular text-text-tertiary">
            {t("conversations.reach", {
              customers: formatNumber(impact.customers),
              companies: formatNumber(impact.companies),
              revenue: money(impact.revenue),
            })}
          </p>
        ) : null}
        <div className="flex min-h-8 flex-wrap items-center gap-x-4 gap-y-1">
          <ReplyAll workspace={workspace} ticket={ticket} disabled={rows.length === 0} />
          <LinkButton size="xs" variant="secondary" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
            {t("conversations.add")}
          </LinkButton>
        </div>
        {loading ? (
          <div className="flex flex-col gap-2">
            {["a", "b", "c"].map((id) => (
              <Skeleton key={id} className="h-6 rounded-lg" />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <Nothing>{t("conversations.empty")}</Nothing>
        ) : (
          <ul aria-label={t("conversations.title")} className="-mx-1.5 flex flex-col">
            {shown.map((row) => (
              <Report key={row.link_id} view={view} row={row} onUnlink={() => setUnlinking(row)} />
            ))}
          </ul>
        )}
        {rows.length > shownFirst ? (
          <div>
            <LinkButton
              size="xs"
              variant="secondary"
              leadingIcon={all ? RiArrowUpSLine : RiArrowDownSLine}
              aria-expanded={all}
              onClick={() => setAll((v) => !v)}
            >
              {all ? t("conversations.fewer") : t("conversations.all", { count: rows.length })}
            </LinkButton>
          </div>
        ) : null}
      </div>
      {adding ? (
        <AddConversations
          workspace={workspace}
          ticket={ticket}
          linked={new Set(rows.map((r) => r.conversation_id))}
          onDone={() => {
            refresh();
            setAdding(false);
          }}
          onClose={() => setAdding(false)}
        />
      ) : null}
      <AlertDialog
        isOpen={unlinking !== null}
        onOpenChange={(open) => (open ? undefined : setUnlinking(null))}
        title={t("conversations.unlinkTitle", { number: unlinking?.conversation_number ?? "" })}
        description={t("conversations.unlinkDescription", {
          name: unlinking ? customerName(unlinking) : "",
          reference: ticket.reference,
        })}
        confirmLabel={t("conversations.unlinkConfirm")}
        onConfirm={async () => {
          if (!unlinking) {
            return;
          }
          await removeTicketLink(workspace, unlinking.link_id).catch((error: Error) => toast.error(error.message));
          refresh();
        }}
      />
    </Section>
  );
}

/*
 * AddConversations picks conversations from the inbox to link, searched
 * by words or number; the ones already linked show as such.
 */
function AddConversations({
  workspace,
  ticket,
  linked,
  onDone,
  onClose,
}: {
  workspace: string;
  ticket: Ticket;
  linked: Set<string>;
  onDone: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation("trackers");
  const [search, setSearch] = useState("");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const { data: found = [], isFetching } = useQuery({
    ...conversationsQuery(workspace, { view: "all", status: "any", search, limit: 30 }),
    placeholderData: (previous) => previous,
  });
  const link = useMutation({
    mutationFn: async () => {
      for (const id of picked) {
        await addTicketLink(workspace, ticket.id, { kind: "tracker", conversation_id: id });
      }
    },
    onSuccess: () => {
      toast.success(t("conversations.linked", { count: picked.size }));
      onDone();
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader title={t("conversations.addTitle")} description={`${ticket.reference} ${ticket.title}`} />
      <ModalBody className="flex min-h-0 flex-1 flex-col gap-3 pb-4">
        <Input
          aria-label={t("conversations.search")}
          placeholder={t("conversations.searchPlaceholder")}
          leadingIcon={RiSearchLine}
          value={search}
          onChange={setSearch}
          autoFocus
        />
        <ul aria-busy={isFetching || undefined} className="flex min-h-0 flex-1 flex-col overflow-y-auto">
          {found.map((c) => {
            const already = linked.has(c.id);
            return (
              <li key={c.id} className="border-separator-border border-t py-2 first:border-t-0">
                <Checkbox
                  isSelected={already || picked.has(c.id)}
                  isDisabled={already}
                  onChange={(on) =>
                    setPicked((current) => {
                      const next = new Set(current);
                      if (on) {
                        next.add(c.id);
                      } else {
                        next.delete(c.id);
                      }
                      return next;
                    })
                  }
                >
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate text-body-2-medium text-text-primary">
                      {c.contact.name || c.contact.email || t("conversations.untitled")}
                      {already ? (
                        <span className="text-text-tertiary"> · {t("conversations.alreadyLinked")}</span>
                      ) : null}
                    </span>
                    <span className="truncate text-caption-1-regular text-text-tertiary">
                      #{c.number} {c.subject ?? c.last_message?.excerpt ?? ""}
                    </span>
                  </span>
                </Checkbox>
              </li>
            );
          })}
          {found.length === 0 && !isFetching ? (
            <li className="py-6 text-center text-body-2-regular text-text-tertiary">{t("conversations.noMatches")}</li>
          ) : null}
        </ul>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button loading={link.isPending} disabled={picked.size === 0} onClick={() => link.mutate()}>
          {t("conversations.linkPicked", { count: picked.size })}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
