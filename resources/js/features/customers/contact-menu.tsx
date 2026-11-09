/*
 * What can be done to one customer from their row: edit them, export a
 * copy of their data and erase them, each in a modal. Export and erase
 * show only to those who may answer privacy requests.
 */
import { RiDeleteBin6Line, RiDownload2Line, RiEditLine } from "@remixicon/react";

import { usePermission } from "@/features/settings/inboxes";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import type { Contact } from "@/lib/api/customers";

import { EraseDialog } from "./erase-customer";
import { ExportDialog } from "./export-customer";
import { personName, useWorkspace } from "./names";

export type ContactDoing = { kind: "edit" | "export" | "erase"; contact: Contact };

export function ContactMenu({ contact, onPick }: { contact: Contact; onPick: (doing: ContactDoing) => void }) {
  const canErase = usePermission(useWorkspace(), "contacts:erase");
  const actions: RowAction<ContactDoing["kind"]>[] = [
    { kind: "edit", label: "Edit", icon: RiEditLine },
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
 * ContactDoings shows the export or erase modal open on a customer, if
 * any; editing is kept in the address and shown by the page.
 */
export function ContactDoings({ doing, onClose }: { doing: ContactDoing | null; onClose: () => void }) {
  if (doing?.kind === "export") {
    return <ExportDialog contact={doing.contact} onClose={onClose} />;
  }
  if (doing?.kind === "erase") {
    return <EraseDialog contact={doing.contact} onClose={onClose} />;
  }

  return null;
}
