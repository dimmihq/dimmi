/*
 * Everyone the team has talked to or been told about, most recently seen
 * first. A teammate searches by name, address or phone, narrows by kind,
 * company, tag, when they were seen or added and one attribute, sorts,
 * picks the columns, and loads more as they go. Blocked customers stay
 * out of the list and have a tab of their own. New contact adds someone
 * the team met elsewhere.
 */
import { RiAddLine, RiSearchLine, RiUser3Line } from "@remixicon/react";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useDeferredValue, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { EmptyState } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";
import { createContact, customerKeys, type PeopleFilters, peopleQuery } from "@/lib/api/customers";

import { CustomersFrame } from "./frame";
import { useWorkspace } from "./names";
import { PeopleFiltersBar } from "./people-filters";
import { ColumnsChooser, PeopleTable } from "./people-table";

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
          <PeopleFiltersBar filters={filters} onChange={setFilters} />
          <ColumnsChooser />
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
