/*
 * The workspace's requests to erase customers and where each stands, in
 * the data table block, a numbered page at a time from the server,
 * narrowed to one status when the teammate picks one. A request still in its grace
 * period can be canceled, which brings the customer back; addresses
 * someone else took meanwhile are named so a teammate can merge the two.
 */
import { RiShieldUserLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { EmptyState } from "@/features/shell/page";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { type ContactErasure, cancelErasure, erasuresPageQuery, peopleCap, peoplePerPage } from "@/lib/api/customers";
import { formatDate, formatNumber } from "@/lib/i18n/format";

import { useWorkspace } from "./names";
import { Muted } from "./people-cells";

const statuses: Record<ContactErasure["status"], { text: string; color: ChipColor }> = {
  pending: { text: "Waiting", color: "yellow" },
  pending_hold: { text: "Held by a retention hold", color: "orange" },
  completed: { text: "Erased", color: "neutral" },
  canceled: { text: "Canceled", color: "soft" },
};

const statusChoices: [string, string][] = [
  ["all", "All statuses"],
  ...Object.entries(statuses).map(([id, s]): [string, string] => [id, s.text]),
];

const widths: Record<string, string> = {
  customer: "w-[300px]",
  status: "w-[220px]",
  wiped: "w-[160px]",
  requested: "w-[160px]",
  actions: "w-[180px]",
};

function CancelButton({ erasure, onCanceled }: { erasure: ContactErasure; onCanceled: (taken: string[]) => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const cancel = useMutation({
    mutationFn: () => cancelErasure(workspace, erasure.id),
    onSuccess: async (canceled) => {
      onCanceled(canceled.identities_conflicted ?? []);
      await client.invalidateQueries({ queryKey: ["workspace", workspace] });
    },
  });
  if (erasure.status !== "pending" && erasure.status !== "pending_hold") {
    return null;
  }

  return (
    <Button variant="secondary" loading={cancel.isPending} onClick={() => cancel.mutate()}>
      Cancel erasure
    </Button>
  );
}

export function ErasuresPage() {
  const workspace = useWorkspace();
  const [status, setStatus] = useState<ContactErasure["status"]>();
  const [page, setPage] = useState(1);
  const list = useQuery({ ...erasuresPageQuery(workspace, status, page), enabled: workspace !== "" });
  const [conflicts, setConflicts] = useState<string[]>([]);
  const shown = list.data?.items ?? [];
  const total = list.data?.total;
  const columns: ColumnDef<ContactErasure>[] = [
    {
      id: "customer",
      header: "Customer",
      cell: ({ row }) => (
        <span dir="ltr" className="font-mono text-body-2-regular text-text-secondary">
          {row.original.contact_id}
        </span>
      ),
    },
    {
      id: "status",
      header: "Status",
      cell: ({ row }) => (
        <Chip variant="caption" color={statuses[row.original.status].color}>
          {statuses[row.original.status].text}
        </Chip>
      ),
    },
    {
      id: "wiped",
      header: "Wiped on",
      cell: ({ row }) => <Muted>{formatDate(row.original.completed_at ?? row.original.scheduled_for)}</Muted>,
    },
    { id: "requested", header: "Requested", cell: ({ row }) => <Muted>{formatDate(row.original.created_at)}</Muted> },
    {
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      cell: ({ row }) => <CancelButton erasure={row.original} onCanceled={setConflicts} />,
    },
  ];

  if (list.isError) {
    return (
      <EmptyState
        icon={RiShieldUserLine}
        title="Erasures could not be loaded"
        description="Only people who may erase customers see this list."
      />
    );
  }

  return (
    <div className="flex h-full flex-col gap-3 px-4 pb-4">
      {conflicts.length > 0 ? (
        <p role="status" className="text-body-2-regular text-text-secondary">
          Brought back, except {conflicts.join(", ")}, which another contact now has.
        </p>
      ) : null}
      <div className="min-h-0 flex-1">
        <DataTable
          aria-label="Erasures"
          data={shown}
          columns={columns}
          getRowId={(erasure) => erasure.id}
          total={total === undefined ? "—" : `${formatNumber(total)} ${total === 1 ? "request" : "requests"}`}
          toolbar={
            <Select
              aria-label="Status"
              className="w-48"
              selectedKey={status ?? "all"}
              onSelectionChange={(key) => {
                setStatus(key === "all" || key === null ? undefined : (key as ContactErasure["status"]));
                setPage(1);
              }}
            >
              {statusChoices.map(([id, name]) => (
                <SelectItem key={id} id={id} textValue={name}>
                  {name}
                </SelectItem>
              ))}
            </Select>
          }
          page={page}
          pageCount={Math.ceil(Math.min(total ?? 0, peopleCap) / peoplePerPage)}
          onPageChange={setPage}
          widths={widths}
          loading={list.isPending}
          skeleton={<TableSkeleton rows={3} />}
          empty={status ? "No request has this status." : "No one has been erased."}
        />
      </div>
    </div>
  );
}
