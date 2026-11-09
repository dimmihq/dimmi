/*
 * The people list's columns, drawn in the data table block. The person
 * comes first, with their address under their name; the rest are picked
 * by each teammate, in a fixed order, and saved with their inbox
 * preferences so every device shows the same table. Name, last seen and
 * added sort the list on the server, each in its one useful direction.
 */
import type { ColumnDef } from "@tanstack/react-table";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { useViewPreferences } from "@/features/inbox/views/use-view-preferences";
import type { Contact } from "@/lib/api/customers";
import { formatDate } from "@/lib/i18n/format";

import { useWorkspace } from "./names";
import { Company, Muted, Person, Seen, Tags } from "./people-cells";
import { sortOf } from "./people-params";

export type ColumnKey = "email" | "phone" | "company" | "kind" | "user_id" | "tags" | "last_seen" | "created";

export const choices: { id: ColumnKey; label: string }[] = [
  { id: "email", label: "Email" },
  { id: "company", label: "Company" },
  { id: "kind", label: "Type" },
  { id: "tags", label: "Tags" },
  { id: "phone", label: "Phone" },
  { id: "user_id", label: "User id" },
  { id: "last_seen", label: "Last seen" },
  { id: "created", label: "Added" },
];

const defaultColumns: ColumnKey[] = ["email", "company", "kind", "tags", "last_seen"];

const cells: Record<ColumnKey, (person: Contact) => ReactNode> = {
  company: (person) => <Company person={person} />,
  kind: (person) => (
    <Chip variant="caption" color={person.kind === "user" ? "blue" : "soft"}>
      {person.kind === "user" ? "User" : "Lead"}
    </Chip>
  ),
  tags: (person) => <Tags tags={person.tags} />,
  email: (person) => <Muted>{person.email ?? "—"}</Muted>,
  phone: (person) => <Muted ltr>{person.phone ?? "—"}</Muted>,
  user_id: (person) => <Muted ltr>{person.external_id ?? "—"}</Muted>,
  last_seen: (person) => <Seen at={person.last_seen_at} />,
  created: (person) => (
    <span className="whitespace-nowrap text-body-medium text-text-primary">{formatDate(person.created_at)}</span>
  ),
};

export function usePeopleColumns() {
  const workspace = useWorkspace();
  const { preference, save } = useViewPreferences(workspace, "people");
  const chosen = (preference.columns ?? defaultColumns).filter((id): id is ColumnKey =>
    choices.some((column) => column.id === id),
  );
  const toggle = (id: ColumnKey, on: boolean) => {
    const next = choices.map((column) => column.id).filter((key) => (key === id ? on : chosen.includes(key)));
    save({ columns: next });
  };
  const columns: ColumnDef<Contact>[] = [
    { id: "name", header: "Name", enableSorting: true, cell: ({ row }) => <Person person={row.original} /> },
    ...choices
      .filter((column) => chosen.includes(column.id))
      .map(
        (column): ColumnDef<Contact> => ({
          id: column.id,
          header: column.label,
          enableSorting: column.id in sortOf,
          cell: ({ row }) => cells[column.id](row.original),
        }),
      ),
  ];

  return { chosen, toggle, columns };
}
