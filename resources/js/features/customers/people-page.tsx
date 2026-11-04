/*
 * Everyone the team has talked to or been told about, most recently seen
 * first. A teammate searches by name, address or phone, narrows by kind
 * or tag, sorts, and loads more as they go. Blocked customers stay out of
 * the list and have a tab of their own. New contact adds someone the team
 * met elsewhere.
 */
import { RiAddLine, RiSearchLine, RiUser3Line } from "@remixicon/react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type Key, useDeferredValue, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { avatarColor } from "@/features/shell/avatar-color";
import { EmptyState } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";
import { type Contact, createContact, customerKeys, type PeopleFilters, peopleQuery } from "@/lib/api/customers";
import { tagsQuery } from "@/lib/api/tags";

import { CustomersFrame } from "./frame";
import { personName, useWorkspace } from "./names";

const any = "any";

const kinds = [
  { id: any, label: "Everyone" },
  { id: "user", label: "Users" },
  { id: "lead", label: "Leads" },
];

const sorts: Record<NonNullable<PeopleFilters["sort"]>, string> = {
  last_seen_at: "Last seen",
  "-created_at": "Newest",
  name: "Name",
};

function seen(iso: string | null): string {
  return iso ? new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short" }) : "—";
}

function NewContactDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const create = useMutation({
    mutationFn: () => createContact(workspace, { name: name.trim() || undefined, email: email.trim() || undefined }),
    onSuccess: async (contact) => {
      await client.invalidateQueries({ queryKey: customerKeys.contacts(workspace) });
      setName("");
      setEmail("");
      await navigate({ to: "/d/customers/$contact", params: { contact: contact.id } });
    },
  });
  const fields = create.error instanceof ApiError ? create.error.fields : {};

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          create.reset();
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title="New contact"
      description="Add someone the team met outside dimmi."
      confirmLabel="Add contact"
      isConfirmDisabled={name.trim() === "" && email.trim() === ""}
      onConfirm={() => create.mutateAsync()}
    >
      <div className="flex flex-col gap-3">
        <Input label="Name" value={name} onChange={setName} isInvalid={"name" in fields} errorMessage={fields.name} />
        <Input
          label="Email"
          type="email"
          inputDir="ltr"
          value={email}
          onChange={setEmail}
          isInvalid={"email" in fields}
          errorMessage={fields.email}
        />
      </div>
    </AlertDialog>
  );
}

function Filters({ filters, onChange }: { filters: PeopleFilters; onChange: (next: PeopleFilters) => void }) {
  const workspace = useWorkspace();
  const { data: tags = [] } = useQuery(tagsQuery(workspace));
  const pick = (apply: (value: string) => PeopleFilters) => (key: Key | null) => {
    if (key !== null) {
      onChange(apply(String(key)));
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select
        aria-label="Kind"
        size="sm"
        className="w-32"
        selectedKey={filters.kind ?? any}
        onSelectionChange={pick((kind) => ({ ...filters, kind: kind === any ? undefined : (kind as "user" | "lead") }))}
      >
        {kinds.map((kind) => (
          <SelectItem key={kind.id} id={kind.id}>
            {kind.label}
          </SelectItem>
        ))}
      </Select>
      <Select
        aria-label="Tag"
        size="sm"
        className="w-40"
        selectedKey={filters.tag ?? any}
        onSelectionChange={pick((tag) => ({ ...filters, tag: tag === any ? undefined : tag }))}
      >
        {[{ id: any, name: "Any tag" }, ...tags].map((tag) => (
          <SelectItem key={tag.id} id={tag.id} textValue={tag.name}>
            {tag.name}
          </SelectItem>
        ))}
      </Select>
      <Select
        aria-label="Sort"
        size="sm"
        className="w-36"
        selectedKey={filters.sort ?? "last_seen_at"}
        onSelectionChange={pick((sort) => ({ ...filters, sort: sort as PeopleFilters["sort"] }))}
      >
        {Object.entries(sorts).map(([id, label]) => (
          <SelectItem key={id} id={id}>
            {label}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}

function PeopleTable({ people, blocked }: { people: Contact[]; blocked: boolean }) {
  return (
    <Table aria-label={blocked ? "Blocked people" : "People"} size="sm" className="[&_th]:border-t-0">
      <TableHeader>
        <TableColumn isRowHeader>Name</TableColumn>
        <TableColumn>Email</TableColumn>
        <TableColumn>Company</TableColumn>
        <TableColumn>Tags</TableColumn>
        <TableColumn>Last seen</TableColumn>
      </TableHeader>
      <TableBody
        items={people}
        renderEmptyState={() => (
          <p className="py-2 text-body-2-regular text-text-tertiary">
            {blocked ? "No one is blocked." : "No one matches."}
          </p>
        )}
      >
        {(person) => (
          <TableRow id={person.id}>
            <TableCell>
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
            </TableCell>
            <TableCell>
              <span dir="auto" className="text-body-2-regular text-text-secondary">
                {person.email ?? "—"}
              </span>
            </TableCell>
            <TableCell>
              <span className="text-body-2-regular text-text-secondary">{person.company?.name ?? "—"}</span>
            </TableCell>
            <TableCell>
              <span className="flex flex-wrap gap-1">
                {person.tags.map((tag) => (
                  <TagChip key={tag.id} tag={tag} />
                ))}
              </span>
            </TableCell>
            <TableCell>
              <span className="whitespace-nowrap text-body-2-regular text-text-secondary">
                {seen(person.last_seen_at)}
              </span>
            </TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
}

export function PeoplePage({ blocked = false }: { blocked?: boolean }) {
  const workspace = useWorkspace();
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<PeopleFilters>({});
  const [adding, setAdding] = useState(false);
  const q = useDeferredValue(search.trim());
  const list = useInfiniteQuery({
    ...peopleQuery(workspace, { ...filters, q: q || undefined, blocked }),
    enabled: workspace !== "",
  });
  const people = list.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <CustomersFrame
      actions={
        blocked ? null : (
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
            New contact
          </Button>
        )
      }
    >
      <div className="flex flex-col gap-3 px-4 pb-4">
        <div className="flex flex-wrap items-center gap-2">
          <Input
            aria-label="Search people"
            size="small"
            className="w-64"
            leadingIcon={RiSearchLine}
            placeholder="Search by name, email or phone"
            value={search}
            onChange={setSearch}
          />
          <Filters filters={filters} onChange={setFilters} />
        </div>
        {list.isError ? (
          <EmptyState icon={RiUser3Line} title="People could not be loaded" description={list.error.message} />
        ) : (
          <PeopleTable people={people} blocked={blocked} />
        )}
        {list.hasNextPage ? (
          <Button
            variant="secondary"
            size="small"
            className="self-center"
            disabled={list.isFetchingNextPage}
            onClick={() => list.fetchNextPage()}
          >
            Load more
          </Button>
        ) : null}
      </div>
      <NewContactDialog isOpen={adding} onClose={() => setAdding(false)} />
    </CustomersFrame>
  );
}
