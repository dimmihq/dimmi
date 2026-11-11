/*
 * Everyone a sent broadcast went to, a page at a time, narrowed to what
 * happened to them: delivered, opened, clicked, replied, unsubscribed,
 * or why it never reached them. A Slack broadcast lists channels rather
 * than people. A recipient who answered links to the conversation the
 * reply opened. Picking an account in the report narrows the list to
 * it.
 */
import { RiChat3Line, RiCloseLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Section } from "@/features/settings/layout";
import {
  type Broadcast,
  type BroadcastCounts,
  type BroadcastRecipient,
  broadcastRecipientsQuery,
  type RecipientFilter,
  type RecipientStatus,
  recipientsPerPage,
} from "@/lib/api/broadcasts";
import { formatNumber } from "@/lib/i18n/format";

import { Pick } from "./controls";
import { counted, reasonName, roomName } from "./looks";

const statusLook: Record<RecipientStatus, { name: string; tint: ChipColor }> = {
  queued: { name: "Waiting", tint: "gray" },
  sent: { name: "Sent", tint: "blue" },
  delivered: { name: "Delivered", tint: "lime" },
  failed: { name: "Failed", tint: "rose" },
  bounced: { name: "Bounced", tint: "rose" },
  complained: { name: "Reported spam", tint: "rose" },
  suppressed: { name: "Suppressed", tint: "gray" },
  skipped: { name: "Left out", tint: "gray" },
  recalled: { name: "Recalled", tint: "gray" },
};

const filters: { id: RecipientFilter | "all"; name: string; count: (counts: BroadcastCounts) => number }[] = [
  { id: "all", name: "Everyone", count: (c) => c.total },
  { id: "delivered", name: "Delivered", count: (c) => c.delivered },
  { id: "opened", name: "Opened", count: (c) => c.opened },
  { id: "clicked", name: "Clicked", count: (c) => c.clicked },
  { id: "replied", name: "Replied", count: (c) => c.replied },
  { id: "unsubscribed", name: "Unsubscribed", count: (c) => c.unsubscribed },
  { id: "bounced", name: "Bounced", count: (c) => c.bounced },
  { id: "complained", name: "Reported spam", count: (c) => c.complained },
  { id: "failed", name: "Failed", count: (c) => c.failed },
  { id: "skipped", name: "Left out", count: (c) => c.skipped },
  { id: "suppressed", name: "Suppressed", count: (c) => c.suppressed },
  { id: "queued", name: "Waiting", count: (c) => c.queued },
];

function did(recipient: BroadcastRecipient): string {
  const done = [
    recipient.opened_at ? "Opened" : "",
    recipient.clicked_at ? "Clicked" : "",
    recipient.replied_at ? "Replied" : "",
    recipient.reactions ? `${formatNumber(recipient.reactions)} reactions` : "",
    recipient.unsubscribed_at ? "Unsubscribed" : "",
    recipient.converted_at ? "Reached the goal" : "",
  ].filter(Boolean);

  return done.join(", ") || "—";
}

function Who({ recipient }: { recipient: BroadcastRecipient }) {
  if (recipient.room_id) {
    return <span className="truncate text-body-2-medium text-text-primary">{roomName(recipient.room_name)}</span>;
  }

  return (
    <span className="flex min-w-0 flex-col">
      <span className="truncate text-body-2-medium text-text-primary">
        {recipient.contact_name || recipient.contact_email || recipient.address || "Unknown person"}
      </span>
      {recipient.contact_name && (recipient.contact_email || recipient.address) ? (
        <span className="truncate text-caption-1-regular text-text-tertiary">
          {recipient.contact_email || recipient.address}
        </span>
      ) : null}
    </span>
  );
}

const columns: ColumnDef<BroadcastRecipient>[] = [
  { id: "who", header: "Recipient", cell: ({ row }) => <Who recipient={row.original} /> },
  {
    id: "account",
    header: "Account",
    cell: ({ row }) => (
      <span className="truncate text-body-2-regular text-text-secondary">{row.original.company_name || "—"}</span>
    ),
  },
  {
    id: "status",
    header: "Status",
    cell: ({ row }) => (
      <span className="flex min-w-0 flex-col items-start gap-0.5">
        <Chip variant="caption" color={statusLook[row.original.status].tint}>
          {statusLook[row.original.status].name}
        </Chip>
        {row.original.reason ? (
          <span className="truncate text-caption-1-regular text-text-tertiary">{reasonName(row.original.reason)}</span>
        ) : null}
      </span>
    ),
  },
  {
    id: "did",
    header: "What they did",
    cell: ({ row }) => <span className="truncate text-body-2-regular text-text-secondary">{did(row.original)}</span>,
  },
  {
    id: "conversation",
    header: "",
    cell: ({ row }) =>
      row.original.conversation_id ? (
        <Link
          to="/d/inbox/$view/$conversation"
          params={{ view: "all", conversation: row.original.conversation_id }}
          className="flex items-center gap-1 rounded-lg px-2 py-1 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiChat3Line aria-hidden className="size-4 text-foreground-icon-tertiary" />
          Conversation
        </Link>
      ) : null,
  },
];

const widths = {
  who: "w-[260px]",
  account: "w-[180px]",
  status: "w-[150px]",
  did: "w-[200px]",
  conversation: "w-[150px]",
};

/*
 * Account is the account the list is narrowed to, with how many of its
 * recipients there are.
 */
export interface Account {
  id: string;
  name: string;
  total: number;
}

export function Recipients({
  workspace,
  broadcast,
  counts,
  account,
  onClearAccount,
}: {
  workspace: string;
  broadcast: Broadcast;
  counts: BroadcastCounts;
  account: Account | null;
  onClearAccount: () => void;
}) {
  const [filter, setFilter] = useState<RecipientFilter | "all">("all");
  const [page, setPage] = useState(1);
  const status = filter === "all" ? undefined : filter;
  const { data: rows = [], isPending } = useQuery(
    broadcastRecipientsQuery(workspace, broadcast.id, { status, company: account?.id, page }),
  );
  const chosen = filters.find((option) => option.id === filter) ?? filters[0];
  const total = account && filter === "all" ? account.total : (chosen?.count(counts) ?? 0);
  const [one, many] = broadcast.channel === "slack" ? ["channel", "channels"] : ["recipient", "recipients"];

  return (
    <Section title={broadcast.channel === "slack" ? "Channels" : "Recipients"}>
      <div className="min-h-[360px] py-3 pe-3">
        <DataTable
          aria-label="Recipients"
          data={rows}
          columns={columns}
          getRowId={(recipient) => recipient.id}
          total={counted(total, one, many)}
          toolbar={
            <div className="flex items-center gap-2">
              {account ? (
                <Button variant="secondary" size="xs" trailingIcon={RiCloseLine} onClick={onClearAccount}>
                  {account.name}
                </Button>
              ) : null}
              <Pick
                label="Show"
                options={filters.map((option) => ({ id: option.id, name: option.name }))}
                value={filter}
                onChange={(id) => {
                  setFilter((id ?? "all") as RecipientFilter | "all");
                  setPage(1);
                }}
              />
            </div>
          }
          page={page}
          pageCount={Math.max(1, Math.ceil(total / recipientsPerPage))}
          onPageChange={setPage}
          widths={widths}
          loading={isPending}
          empty="Nobody here."
        />
      </div>
    </Section>
  );
}
