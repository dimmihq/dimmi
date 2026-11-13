/*
 * The customers who reported what a tracker follows, as a compact list
 * in its details: each linked conversation's customer, their company
 * and the state of their own ticket, opening that conversation in the
 * inbox. One quiet line above says who it reaches in all. Teammates
 * link more conversations from a picker of the inbox's own
 * conversations, unlink one after confirming, and reply to all of them
 * at once from the "Reply all" button.
 */
import { RiAddLine, RiLinkUnlinkM, RiReplyAllLine, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { toast } from "@/components/base/toast/toast";
import { conversationsQuery } from "@/lib/api/inbox";
import { type Impact, type TrackerConversation, trackerKeys } from "@/lib/api/ticket-trackers";
import {
  addTicketLink,
  removeTicketLink,
  type Ticket,
  type TicketState,
  ticketKeys,
  ticketStatesQuery,
} from "@/lib/api/tickets";
import { formatNumber } from "@/lib/i18n/format";

import { StatePill } from "./state-pill";
import { customerName, money } from "./tracker-helpers";

/*
 * shownFirst is how many reports show before "Show all".
 */
const shownFirst = 8;

/*
 * Report is one customer's report: who, their company and their own
 * ticket's state, opening the conversation; unlinking shows on hover.
 */
function Report({
  view,
  row,
  state,
  onUnlink,
}: {
  view: string;
  row: TrackerConversation;
  state: TicketState | undefined;
  onUnlink: () => void;
}) {
  const { t } = useTranslation("trackers");

  return (
    <li className="group flex items-center gap-1">
      <Link
        to="/d/inbox/$view/$conversation"
        params={{ view, conversation: row.conversation_id }}
        title={t("conversations.open", { number: row.conversation_number })}
        className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1.5 py-1 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-body-2-medium text-text-primary">{customerName(row)}</span>
          <span className="truncate text-caption-1-regular text-text-tertiary">
            {row.company_name ?? t("conversations.noCompany")}
          </span>
        </span>
        {state ? <StatePill state={state} /> : null}
      </Link>
      <IconButton
        size="small"
        icon={RiLinkUnlinkM}
        aria-label={t("conversations.unlink", { number: row.conversation_number })}
        className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
        onClick={onUnlink}
      />
    </li>
  );
}

/*
 * CustomerReports is the block in the tracker's details: the count, who
 * it reaches, the reports, and the ways to link more and reply to all.
 */
export function CustomerReports({
  workspace,
  view,
  ticket,
  rows,
  impact,
  loading,
  onReplyAll,
}: {
  workspace: string;
  view: string;
  ticket: Ticket;
  rows: TrackerConversation[];
  impact: Impact | undefined;
  loading: boolean;
  onReplyAll: () => void;
}) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const { data: allStates = [] } = useQuery(ticketStatesQuery(workspace));
  const states = new Map(allStates.map((s) => [s.id, s]));
  const [adding, setAdding] = useState(false);
  const [all, setAll] = useState(false);
  const [unlinking, setUnlinking] = useState<TrackerConversation | null>(null);
  const refresh = () => {
    void client.invalidateQueries({ queryKey: trackerKeys.conversations(workspace, ticket.id) });
    void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
  };
  const shown = all ? rows : rows.slice(0, shownFirst);

  return (
    <section
      aria-label={t("conversations.title")}
      className="flex flex-col gap-2 border-t border-separator-border pt-3"
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-caption-1-medium text-text-tertiary">
          {t("conversations.title")} · {formatNumber(rows.length)}
        </h3>
        <Button
          size="xs"
          variant="secondary"
          leadingIcon={RiReplyAllLine}
          disabled={rows.length === 0}
          onClick={onReplyAll}
        >
          {t("conversations.replyAll")}
        </Button>
      </div>
      {impact && impact.customers > 0 ? (
        <p className="text-caption-1-regular text-text-secondary">
          {t("conversations.reach", {
            customers: formatNumber(impact.customers),
            companies: formatNumber(impact.companies),
            revenue: money(impact.revenue),
          })}
        </p>
      ) : null}
      {loading ? (
        <div className="flex flex-col gap-2">
          {["a", "b", "c"].map((id) => (
            <Skeleton key={id} className="h-9 rounded-lg" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <p className="text-body-2-regular text-text-tertiary">{t("conversations.empty")}</p>
      ) : (
        <ul aria-label={t("conversations.title")} className="-mx-1.5 flex flex-col">
          {shown.map((row) => (
            <Report
              key={row.link_id}
              view={view}
              row={row}
              state={row.ticket_state_id ? states.get(row.ticket_state_id) : undefined}
              onUnlink={() => setUnlinking(row)}
            />
          ))}
        </ul>
      )}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <LinkButton size="xs" variant="secondary" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
          {t("conversations.add")}
        </LinkButton>
        {rows.length > shownFirst ? (
          <LinkButton size="xs" variant="secondary" onClick={() => setAll((v) => !v)}>
            {all ? t("conversations.fewer") : t("conversations.all", { count: rows.length })}
          </LinkButton>
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
    </section>
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
