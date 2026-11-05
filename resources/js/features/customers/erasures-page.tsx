/*
 * The workspace's requests to erase customers and where each stands. A
 * request still in its grace period can be canceled, which brings the
 * customer back; addresses someone else took meanwhile are named so a
 * teammate can merge the two.
 */
import { RiShieldUserLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { EmptyState } from "@/features/shell/page";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { type ContactErasure, cancelErasure, erasuresQuery } from "@/lib/api/customers";
import { CustomersFrame } from "./frame";
import { useWorkspace } from "./names";

const statusText: Record<ContactErasure["status"], string> = {
  pending: "Waiting",
  pending_hold: "Held by a retention hold",
  completed: "Erased",
  canceled: "Canceled",
};

function day(iso: string | null): string {
  return iso ? new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" }) : "—";
}

export function ErasuresPage() {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const query = erasuresQuery(workspace);
  const { data: erasures, isError } = useQuery({ ...query, enabled: workspace !== "" });
  const [conflicts, setConflicts] = useState<string[]>([]);
  const cancel = useMutation({
    mutationFn: (id: string) => cancelErasure(workspace, id),
    onSuccess: async (canceled) => {
      setConflicts(canceled.identities_conflicted ?? []);
      await client.invalidateQueries({ queryKey: ["workspace", workspace] });
    },
  });

  return (
    <CustomersFrame>
      <div className="flex flex-col gap-3 px-4 pb-4">
        {conflicts.length > 0 ? (
          <p role="status" className="text-body-2-regular text-text-secondary">
            Brought back, except {conflicts.join(", ")}, which another contact now has.
          </p>
        ) : null}
        {isError ? (
          <EmptyState
            icon={RiShieldUserLine}
            title="Erasures could not be loaded"
            description="Only people who may erase customers see this list."
          />
        ) : (
          <Table aria-label="Erasures" size="sm" className="[&_th]:border-t-0">
            <TableHeader>
              <TableColumn isRowHeader>Customer</TableColumn>
              <TableColumn>Status</TableColumn>
              <TableColumn>Wiped on</TableColumn>
              <TableColumn>Requested</TableColumn>
              <TableColumn>
                <span className="sr-only">Actions</span>
              </TableColumn>
            </TableHeader>
            <TableBody
              items={erasures ?? []}
              renderEmptyState={() =>
                erasures ? (
                  <p className="py-2 text-body-2-regular text-text-tertiary">No one has been erased.</p>
                ) : (
                  <TableSkeleton rows={3} />
                )
              }
            >
              {(erasure) => (
                <TableRow id={erasure.id}>
                  <TableCell>
                    <span dir="ltr" className="font-mono text-body-2-regular text-text-secondary">
                      {erasure.contact_id}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-body-2-regular text-text-primary">{statusText[erasure.status]}</span>
                  </TableCell>
                  <TableCell>
                    <span className="text-body-2-regular text-text-secondary">
                      {day(erasure.completed_at ?? erasure.scheduled_for)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-body-2-regular text-text-secondary">{day(erasure.created_at)}</span>
                  </TableCell>
                  <TableCell>
                    {erasure.status === "pending" || erasure.status === "pending_hold" ? (
                      <Button
                        variant="secondary"
                        size="small"
                        loading={cancel.isPending}
                        onClick={() => cancel.mutate(erasure.id)}
                      >
                        Cancel erasure
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        )}
      </div>
    </CustomersFrame>
  );
}
