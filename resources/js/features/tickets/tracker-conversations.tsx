/*
 * The conversations a tracker or an incident links, one row each: who
 * wrote, their company with its plan and revenue, the state of their own
 * ticket, when they last wrote, and the way into the conversation. Rows
 * fold their company and ticket under the name on a narrow screen, so
 * the list never scrolls sideways. Teammates link more conversations
 * from a picker of the inbox's own conversations and unlink one after
 * confirming.
 */
import { RiAddLine, RiArrowRightUpLine, RiLinkUnlinkM, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Empty, Section } from "@/features/settings/layout";
import { conversationsQuery } from "@/lib/api/inbox";
import { type TrackerConversation, trackerKeys } from "@/lib/api/ticket-trackers";
import {
  addTicketLink,
  removeTicketLink,
  type Ticket,
  type TicketState,
  ticketKeys,
  ticketStatesQuery,
} from "@/lib/api/tickets";
import { formatRelative } from "@/lib/i18n/format";

import { StatePill } from "./state-pill";
import { customerName, money } from "./tracker-helpers";

const grid =
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_6rem_minmax(0,0.9fr)_7rem_4.5rem]";

function Row({
  row,
  states,
  onUnlink,
}: {
  row: TrackerConversation;
  states: Map<string, TicketState>;
  onUnlink: () => void;
}) {
  const { t } = useTranslation("trackers");
  const state = row.ticket_state_id ? states.get(row.ticket_state_id) : undefined;
  const company = row.company_name ? (
    <span className="flex min-w-0 items-center gap-1.5">
      <span className="truncate text-body-2-regular text-text-secondary">{row.company_name}</span>
      {row.company_plan ? (
        <Chip variant="caption" color="gray" className="shrink-0">
          {row.company_plan}
        </Chip>
      ) : null}
    </span>
  ) : (
    <span className="text-body-2-regular text-text-tertiary">{t("conversations.noCompany")}</span>
  );
  const ticket =
    row.ticket_reference && state ? (
      <span className="flex min-w-0 items-center gap-1.5">
        <span className="shrink-0 text-caption-1-medium text-text-secondary">{row.ticket_reference}</span>
        <StatePill state={state} />
      </span>
    ) : (
      <span className="text-body-2-regular text-text-tertiary">{t("conversations.noTicket")}</span>
    );

  return (
    <li className={`${grid} group border-separator-border border-t py-2.5 pe-3 first:border-t-0`}>
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-2-medium text-text-primary">{customerName(row)}</span>
        <span className="truncate text-caption-1-regular text-text-tertiary">
          #{row.conversation_number} {row.subject ?? t("conversations.untitled")}
        </span>
        <span className="mt-1 flex flex-wrap items-center gap-2 md:hidden">
          {company}
          {ticket}
        </span>
      </span>
      <span className="hidden min-w-0 md:block">{company}</span>
      <span className="hidden text-end text-body-2-medium text-text-primary tabular-nums md:block">
        {row.company_revenue > 0 ? money(row.company_revenue) : "—"}
      </span>
      <span className="hidden min-w-0 md:block">{ticket}</span>
      <span className="hidden text-caption-1-regular text-text-tertiary md:block">
        {row.last_message_at ? formatRelative(row.last_message_at) : "—"}
      </span>
      <span className="flex items-center justify-end gap-0.5">
        <Link
          to="/d/inbox/$view/$conversation"
          params={{ view: "all", conversation: row.conversation_id }}
          aria-label={t("conversations.open", { number: row.conversation_number })}
          title={t("conversations.open", { number: row.conversation_number })}
          className="flex size-8 items-center justify-center rounded-lg text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover hover:text-foreground-icon-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowRightUpLine aria-hidden className="size-4" />
        </Link>
        <IconButton
          size="small"
          icon={RiLinkUnlinkM}
          aria-label={t("conversations.unlink", { number: row.conversation_number })}
          className="md:opacity-0 md:group-hover:opacity-100 md:focus-visible:opacity-100"
          onClick={onUnlink}
        />
      </span>
    </li>
  );
}

/*
 * TrackerConversations lists the linked conversations under a heading,
 * with the button that links more.
 */
export function TrackerConversations({
  workspace,
  ticket,
  rows,
  loading,
}: {
  workspace: string;
  ticket: Ticket;
  rows: TrackerConversation[];
  loading: boolean;
}) {
  const { t } = useTranslation("trackers");
  const client = useQueryClient();
  const { data: allStates = [] } = useQuery(ticketStatesQuery(workspace));
  const states = new Map(allStates.map((s) => [s.id, s]));
  const [adding, setAdding] = useState(false);
  const [unlinking, setUnlinking] = useState<TrackerConversation | null>(null);
  const refresh = () => {
    void client.invalidateQueries({ queryKey: trackerKeys.conversations(workspace, ticket.id) });
    void client.invalidateQueries({ queryKey: ticketKeys.one(workspace, ticket.id) });
  };

  return (
    <>
      <Section
        title={t("conversations.title")}
        description={t("conversations.description")}
        loading={loading}
        rows={4}
        action={
          <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
            {t("conversations.add")}
          </Button>
        }
      >
        {rows.length === 0 ? (
          <Empty>{t("conversations.empty")}</Empty>
        ) : (
          <div className="flex flex-col">
            <div aria-hidden className={`${grid} hidden py-2 pe-3 text-caption-1-medium text-text-tertiary md:grid`}>
              <span>{t("conversations.columns.customer")}</span>
              <span>{t("conversations.columns.company")}</span>
              <span className="text-end">{t("conversations.columns.revenue")}</span>
              <span>{t("conversations.columns.ticket")}</span>
              <span>{t("conversations.columns.lastMessage")}</span>
              <span />
            </div>
            <ul aria-label={t("conversations.title")} className="flex flex-col">
              {rows.map((row) => (
                <Row key={row.link_id} row={row} states={states} onUnlink={() => setUnlinking(row)} />
              ))}
            </ul>
          </div>
        )}
      </Section>
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
    </>
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
