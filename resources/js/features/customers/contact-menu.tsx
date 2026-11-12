/*
 * What can be done to one customer from their row: edit them, merge
 * another record of them in, export a copy of their data and erase
 * them, each in a modal. Merge shows to those who may change customers;
 * export and erase only to those who may answer privacy requests.
 */
import { RiDeleteBin6Line, RiDownload2Line, RiEditLine, RiMergeCellsHorizontal } from "@remixicon/react";

import { usePermission } from "@/features/settings/inboxes";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import type { Contact } from "@/lib/api/customers";

import { EraseDialog } from "./erase-customer";
import { ExportDialog } from "./export-customer";
import { MergeContactDialog } from "./merge-customer";
import { personName, useWorkspace } from "./names";

export type ContactDoing = { kind: "edit" | "merge" | "export" | "erase"; contact: Contact };

export function ContactMenu({ contact, onPick }: { contact: Contact; onPick: (doing: ContactDoing) => void }) {
  const workspace = useWorkspace();
  const canErase = usePermission(workspace, "contacts:erase");
  const canMerge = usePermission(workspace, "contacts:write");
  const actions: RowAction<ContactDoing["kind"]>[] = [
    { kind: "edit", label: "Edit", icon: RiEditLine },
    ...(canMerge ? [{ kind: "merge" as const, label: "Merge…", icon: RiMergeCellsHorizontal }] : []),
    ...(canErase
      ? [
          { kind: "export" as const, label: "Export data", icon: RiDownload2Line },
          { kind: "erase" as const, label: "Erase", icon: RiDeleteBin6Line, danger: true },
        ]
      : []),
  ];

  return <RowMenu label={personName(contact)} actions={actions} onPick={(kind) => onPick({ kind, contact })} />;
}

/*
 * ContactDoings shows the merge, export or erase modal open on a
 * customer, if any; editing is kept in the address and shown by the page.
 */
export function ContactDoings({ doing, onClose }: { doing: ContactDoing | null; onClose: () => void }) {
  if (doing?.kind === "merge") {
    return <MergeContactDialog contact={doing.contact} onClose={onClose} />;
  }
  if (doing?.kind === "export") {
    return <ExportDialog contact={doing.contact} onClose={onClose} />;
  }
  if (doing?.kind === "erase") {
    return <EraseDialog contact={doing.contact} onClose={onClose} />;
  }

  return null;
}
