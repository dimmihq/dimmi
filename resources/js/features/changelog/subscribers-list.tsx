/*
 * The people who asked for the changelog by email, a numbered page at a
 * time: who is active, who still has to confirm from the mail they got,
 * and who left. Removing someone forgets the address altogether.
 */

import { RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import {
  type ChangelogSubscriber,
  changelogKeys,
  deleteSubscriber,
  entriesPerPage,
  subscribersQuery,
} from "@/lib/api/changelog";
import { formatDate, formatNumber } from "@/lib/i18n/format";

const statuses: Record<ChangelogSubscriber["status"], { text: string; color: ChipColor }> = {
  active: { text: "Active", color: "lime" },
  pending: { text: "Waiting to confirm", color: "yellow" },
  unsubscribed: { text: "Unsubscribed", color: "gray" },
};

const choices: [string, string][] = [
  ["all", "Everyone"],
  ["active", "Active"],
  ["pending", "Waiting to confirm"],
  ["unsubscribed", "Unsubscribed"],
];

export function SubscribersList({ workspace, helpCenter }: { workspace: string; helpCenter: string }) {
  const client = useQueryClient();
  const [status, setStatus] = useState<ChangelogSubscriber["status"]>();
  const [page, setPage] = useState(1);
  const [removing, setRemoving] = useState<ChangelogSubscriber | null>(null);
  const list = useQuery(subscribersQuery(workspace, helpCenter, { status, page }));
  const total = list.data?.total ?? 0;
  const remove = useMutation({
    mutationFn: (id: string) => deleteSubscriber(workspace, id),
    onSuccess: () => client.invalidateQueries({ queryKey: changelogKeys.subscribers(workspace, helpCenter) }),
    onError: (error) => toast.error(error.message),
  });
  const columns: ColumnDef<ChangelogSubscriber>[] = [
    {
      id: "email",
      header: "Email",
      cell: ({ row }) => (
        <span dir="ltr" className="truncate text-body-regular text-text-primary">
          {row.original.email}
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
      id: "since",
      header: "Since",
      cell: ({ row }) => (
        <span className="text-body-2-regular text-text-secondary">
          {formatDate(row.original.confirmed_at ?? row.original.created_at)}
        </span>
      ),
    },
    {
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      cell: ({ row }) => (
        <IconButton
          size="small"
          icon={RiDeleteBinLine}
          aria-label={`Remove ${row.original.email}`}
          onClick={() => setRemoving(row.original)}
        />
      ),
    },
  ];

  return (
    <>
      <DataTable
        aria-label="Subscribers"
        data={list.data?.items ?? []}
        columns={columns}
        getRowId={(s) => s.id}
        total={`${formatNumber(total)} ${total === 1 ? "person" : "people"}`}
        toolbar={
          <Select
            aria-label="Status"
            className="w-56"
            selectedKey={status ?? "all"}
            onSelectionChange={(key) => {
              setStatus(key === "all" || key === null ? undefined : (key as ChangelogSubscriber["status"]));
              setPage(1);
            }}
          >
            {choices.map(([id, name]) => (
              <SelectItem key={id} id={id} textValue={name}>
                {name}
              </SelectItem>
            ))}
          </Select>
        }
        page={page}
        pageCount={Math.max(1, Math.ceil(total / entriesPerPage))}
        onPageChange={setPage}
        widths={{ email: "w-[46%]", status: "w-[26%]", since: "w-[20%]", actions: "w-[8%]" }}
        loading={list.isPending}
        skeleton={<TableSkeleton rows={4} />}
        empty={status ? "Nobody here." : "Nobody has subscribed yet. The changelog's page has the form."}
      />
      <AlertDialog
        isOpen={removing !== null}
        onOpenChange={(open) => (open ? undefined : setRemoving(null))}
        title={removing ? `Remove ${removing.email}?` : "Remove subscriber?"}
        description="They stop getting updates and the address is forgotten. They can subscribe again."
        confirmLabel="Remove"
        onConfirm={() => (removing ? remove.mutateAsync(removing.id) : undefined)}
      />
    </>
  );
}
