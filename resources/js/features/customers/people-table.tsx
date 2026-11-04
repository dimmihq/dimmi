/*
 * The people list as a table whose columns each teammate picks. Name is
 * always first; the rest come from the chooser, in a fixed order, and
 * are saved with the person's inbox preferences so every device shows
 * the same table.
 */
import { RiLayoutColumnLine } from "@remixicon/react";
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { useViewPreferences } from "@/features/inbox/views/use-view-preferences";
import { avatarColor } from "@/features/shell/avatar-color";
import type { Contact } from "@/lib/api/customers";

import { personName, useWorkspace } from "./names";

type ColumnKey = "email" | "phone" | "company" | "kind" | "user_id" | "tags" | "last_seen" | "created";

const columns: { id: ColumnKey; label: string }[] = [
  { id: "email", label: "Email" },
  { id: "phone", label: "Phone" },
  { id: "company", label: "Company" },
  { id: "kind", label: "Kind" },
  { id: "user_id", label: "User id" },
  { id: "tags", label: "Tags" },
  { id: "last_seen", label: "Last seen" },
  { id: "created", label: "Added" },
];

const defaultColumns: ColumnKey[] = ["email", "company", "tags", "last_seen"];

function day(iso: string | null): string {
  return iso ? new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short" }) : "—";
}

function Muted({ children, ltr = false }: { children: ReactNode; ltr?: boolean }) {
  return (
    <span dir={ltr ? "ltr" : "auto"} className="whitespace-nowrap text-body-2-regular text-text-secondary">
      {children}
    </span>
  );
}

function cell(column: ColumnKey, person: Contact): ReactNode {
  const values: Record<ColumnKey, () => ReactNode> = {
    email: () => <Muted>{person.email ?? "—"}</Muted>,
    phone: () => <Muted ltr>{person.phone ?? "—"}</Muted>,
    company: () => <Muted>{person.company?.name ?? "—"}</Muted>,
    kind: () => <Muted>{person.kind === "user" ? "User" : "Lead"}</Muted>,
    user_id: () => <Muted ltr>{person.external_id ?? "—"}</Muted>,
    tags: () => (
      <span className="flex flex-wrap gap-1">
        {person.tags.map((tag) => (
          <TagChip key={tag.id} tag={tag} />
        ))}
      </span>
    ),
    last_seen: () => <Muted>{day(person.last_seen_at)}</Muted>,
    created: () => <Muted>{day(person.created_at)}</Muted>,
  };

  return values[column]();
}

function usePeopleColumns() {
  const workspace = useWorkspace();
  const { preference, save } = useViewPreferences(workspace, "people");
  const chosen = (preference.columns ?? defaultColumns).filter((id): id is ColumnKey =>
    columns.some((column) => column.id === id),
  );
  const toggle = (id: ColumnKey, on: boolean) => {
    const next = columns.map((column) => column.id).filter((key) => (key === id ? on : chosen.includes(key)));
    save({ columns: next });
  };

  return { chosen, toggle };
}

export function ColumnsChooser() {
  const { chosen, toggle } = usePeopleColumns();

  return (
    <PopoverTrigger>
      <Button variant="secondary" size="small" leadingIcon={RiLayoutColumnLine}>
        Columns
      </Button>
      <Popover aria-label="Columns" placement="bottom end" className="w-56">
        <div className="flex flex-col gap-2 p-1">
          {columns.map((column) => (
            <Checkbox
              key={column.id}
              size="sm"
              isSelected={chosen.includes(column.id)}
              onChange={(on) => toggle(column.id, on)}
            >
              {column.label}
            </Checkbox>
          ))}
        </div>
      </Popover>
    </PopoverTrigger>
  );
}

export function PeopleTable({ people, blocked }: { people: Contact[]; blocked: boolean }) {
  const { chosen } = usePeopleColumns();
  const shown = [{ id: "name", label: "Name" }, ...columns.filter((column) => chosen.includes(column.id))];

  return (
    <Table aria-label={blocked ? "Blocked people" : "People"} size="sm" className="[&_th]:border-t-0">
      <TableHeader columns={shown}>
        {(column) => (
          <TableColumn id={column.id} isRowHeader={column.id === "name"}>
            {column.label}
          </TableColumn>
        )}
      </TableHeader>
      <TableBody
        items={people}
        dependencies={[chosen.join()]}
        renderEmptyState={() => (
          <p className="py-2 text-body-2-regular text-text-tertiary">
            {blocked ? "No one is blocked." : "No one matches."}
          </p>
        )}
      >
        {(person) => (
          <TableRow id={person.id} columns={shown}>
            {(column) => (
              <TableCell>
                {column.id === "name" ? (
                  <Link
                    to="/d/customers/$contact"
                    params={{ contact: person.id }}
                    className="flex items-center gap-2 rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                  >
                    <Avatar
                      size="xs"
                      color={avatarColor(personName(person))}
                      initials={personName(person).charAt(0).toUpperCase()}
                    />
                    <span className="truncate">{personName(person)}</span>
                  </Link>
                ) : (
                  cell(column.id as ColumnKey, person)
                )}
              </TableCell>
            )}
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
}
