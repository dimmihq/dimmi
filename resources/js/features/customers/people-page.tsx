/*
 * Everyone the team has talked to or been told about, most recently seen
 * first, in the data table block: how many match, a filters dropdown,
 * search and the table's actions (columns and export) above numbered
 * pages the server counts. Name, last seen and added sort it. Ticking
 * people lets a teammate tag them together. Each row's menu edits the
 * person in a modal, kept in the address so it can be shared and
 * reloaded, or exports or erases them. Blocked customers stay out of the list and have a tab of
 * their own. New contact adds someone the team met elsewhere.
 */
import { RiAddLine, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import type { ColumnDef, RowSelectionState } from "@tanstack/react-table";
import { useDeferredValue, useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input, InputBase } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { ExportButton } from "@/features/exports/export-button";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { attributesQuery } from "@/lib/api/attributes";
import { ApiError } from "@/lib/api/client";
import {
  type Contact,
  type ContactChanges,
  createContact,
  customerKeys,
  type PeopleFilters,
  peopleCap,
  peoplePageQuery,
  peoplePerPage,
  peopleSelection,
  updateContact,
} from "@/lib/api/customers";
import { formatNumber } from "@/lib/i18n/format";

import { ContactEditModal } from "./contact-edit";
import { type ContactDoing, ContactDoings, ContactMenu } from "./contact-menu";
import { CustomersActions } from "./frame";
import { useWorkspace } from "./names";
import { usePeopleColumns } from "./people-columns";
import { AddFilter, FilterBar } from "./people-filter-bar";
import { conditions, type Filter, filterFields } from "./people-filter-fields";
import { peopleWidths, sortFrom, sortingOf } from "./people-params";
import { ColumnsChooser } from "./people-table";

function NewContactDialog({ isOpen, onClose }: { isOpen: boolean; onClose: (created?: string) => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const create = useMutation({
    mutationFn: () => createContact(workspace, { name: name.trim() || undefined, email: email.trim() || undefined }),
    onSuccess: async (contact) => {
      await client.invalidateQueries({ queryKey: customerKeys.contacts(workspace) });
      setName("");
      setEmail("");
      onClose(contact.id);
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

/*
 * TagSelected tags the ticked people with the inbox's tag picker: a tag
 * every one of them has shows ticked, picking one adds it to all of them
 * and unticking takes it off all of them. A name nobody has used becomes
 * a new tag. The ticks stay, so several tags can be added in a row.
 */
function TagSelected({ people }: { people: Contact[] }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const shared = (people[0]?.tags ?? []).filter((tag) =>
    people.every((person) => person.tags.some((other) => other.id === tag.id)),
  );
  const change = useMutation({
    mutationFn: (changes: ContactChanges) =>
      Promise.all(people.map((person) => updateContact(workspace, person.id, changes))),
    onSettled: () => client.invalidateQueries({ queryKey: customerKeys.contacts(workspace) }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <TagPicker
      workspace={workspace}
      size="medium"
      label="Tag the selected people"
      selected={shared}
      onAdd={(name) => change.mutate({ add_tags: [name] })}
      onRemove={(tag) => change.mutate({ remove_tags: [tag.id] })}
    />
  );
}

function totalText(total: number | undefined, blocked: boolean): string {
  if (total === undefined) {
    return "—";
  }
  const shown = total >= peopleCap ? `${formatNumber(peopleCap)}+` : formatNumber(total);
  const noun = total === 1 ? "person" : "people";

  return blocked ? `${shown} blocked` : `${shown} ${noun}`;
}

export function PeoplePage({ blocked = false }: { blocked?: boolean }) {
  const workspace = useWorkspace();
  const navigate = useNavigate();
  const { contact: open } = useSearch({ strict: false }) as { contact?: string };
  const [search, setSearch] = useState("");
  const [chips, setChips] = useState<Filter[]>([]);
  const [sort, setSort] = useState<PeopleFilters["sort"]>();
  const { data: attributes = [] } = useQuery({ ...attributesQuery(workspace), enabled: workspace !== "" });
  const fields = filterFields(attributes);
  const [page, setPage] = useState(1);
  const [selection, setSelection] = useState<RowSelectionState>({});
  const [adding, setAdding] = useState(false);
  const q = useDeferredValue(search.trim());
  const current: PeopleFilters = { conditions: conditions(chips, fields), sort, q: q || undefined, blocked };
  const list = useQuery({ ...peoplePageQuery(workspace, current, page), enabled: workspace !== "" });
  const [doing, setDoing] = useState<ContactDoing | null>(null);
  const { columns: chosen } = usePeopleColumns();
  const columns: ColumnDef<Contact>[] = [
    ...chosen,
    {
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      cell: ({ row }) => (
        <span className="flex justify-end">
          <ContactMenu
            contact={row.original}
            onPick={(next) => (next.kind === "edit" ? show(next.contact.id) : setDoing(next))}
          />
        </span>
      ),
    },
  ];
  const people = list.data?.items ?? [];
  const total = list.data?.total;
  const pageCount = Math.ceil(Math.min(total ?? 0, peopleCap) / peoplePerPage);
  const show = (contact: string | undefined) =>
    navigate({ to: ".", search: contact ? { contact } : {}, replace: contact === undefined });
  const restart = () => {
    setPage(1);
    setSelection({});
  };

  return (
    <>
      {blocked ? null : (
        <CustomersActions>
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
            New contact
          </Button>
        </CustomersActions>
      )}
      <div className="h-full px-4 pb-4">
        <DataTable
          aria-label={blocked ? "Blocked people" : "People"}
          data={people}
          columns={columns}
          getRowId={(person) => person.id}
          total={list.isError ? list.error.message : totalText(total, blocked)}
          toolbar={
            <>
              <InputBase
                aria-label="Search people"
                placeholder="Search"
                leadingIcon={RiSearchLine}
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setPage(1);
                  setSelection({});
                }}
                fieldClassName="min-w-[153px] flex-1 rounded-full bg-background-secondary-default sm:w-[200px] sm:min-w-0 sm:flex-none"
                className="text-body-medium"
              />
              <AddFilter
                fields={fields}
                filters={chips}
                onAdd={(filter) => {
                  setChips([...chips, filter]);
                  restart();
                }}
              />
              <div className="flex items-center gap-1.5">
                <ColumnsChooser />
                <ExportButton
                  workspace={workspace}
                  kind="contacts"
                  selection={peopleSelection(current)}
                  compact="table"
                />
              </div>
            </>
          }
          filters={
            chips.length > 0 ? (
              <FilterBar
                fields={fields}
                filters={chips}
                onChange={(next) => {
                  setChips(next);
                  restart();
                }}
              />
            ) : null
          }
          selectionBar={(selected, clear) => (
            <>
              <span className="text-body-medium whitespace-nowrap text-text-secondary">
                {formatNumber(selected.length)} selected
              </span>
              <TagSelected people={people.filter((person) => selected.includes(person.id))} />
              <LinkButton variant="secondary" onClick={clear}>
                Clear
              </LinkButton>
            </>
          )}
          selection={selection}
          onSelectionChange={setSelection}
          sorting={sortingOf(sort)}
          onSortingChange={(sorting) => {
            setSort(sortFrom(sorting));
            restart();
          }}
          page={page}
          pageCount={pageCount}
          onPageChange={(next) => {
            setPage(next);
            setSelection({});
          }}
          onRowAction={(id) => show(id)}
          widths={peopleWidths}
          loading={list.isPending}
          skeleton={<TableSkeleton />}
          empty={blocked ? "No one is blocked." : "No one matches."}
        />
      </div>
      <NewContactDialog
        isOpen={adding}
        onClose={(created) => {
          setAdding(false);
          if (created) {
            void show(created);
          }
        }}
      />
      {open ? <ContactEditModal key={open} id={open} onClose={() => show(undefined)} /> : null}
      <ContactDoings doing={doing} onClose={() => setDoing(null)} />
    </>
  );
}
