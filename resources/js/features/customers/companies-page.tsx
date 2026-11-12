/*
 * The companies the workspace's customers work at, in the data table
 * block: how many match, search by name or domain, numbered pages, and
 * how many people each has. Ticking companies lets a teammate tag them
 * together with the inbox's tag picker. Opening one slides it in from the
 * side, kept in the address. New company adds one by hand.
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
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { ApiError } from "@/lib/api/client";
import {
  type Company,
  companiesPageQuery,
  createCompany,
  peopleCap,
  peoplePerPage,
  updateCompany,
} from "@/lib/api/customers";
import { formatDate, formatNumber } from "@/lib/i18n/format";

import { CompanyDrawer } from "./company-drawer";
import { CustomersActions } from "./frame";
import { ImportButton } from "./import-dialog";
import { useWorkspace } from "./names";
import { CompanyName, Muted, Tags } from "./people-cells";

function NewCompanyDialog({ isOpen, onClose }: { isOpen: boolean; onClose: (created?: string) => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");
  const create = useMutation({
    mutationFn: () => createCompany(workspace, { name: name.trim(), domain: domain.trim() || undefined }),
    onSuccess: async (company) => {
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "companies"] });
      setName("");
      setDomain("");
      onClose(company.id);
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
      title="New company"
      confirmLabel="Add company"
      isConfirmDisabled={name.trim() === ""}
      onConfirm={() => create.mutateAsync()}
    >
      <div className="flex flex-col gap-3">
        <Input label="Name" value={name} onChange={setName} isInvalid={"name" in fields} errorMessage={fields.name} />
        <Input
          label="Domain"
          inputDir="ltr"
          value={domain}
          onChange={setDomain}
          isInvalid={"domain" in fields}
          errorMessage={fields.domain}
        />
      </div>
    </AlertDialog>
  );
}

/*
 * TagSelected tags the ticked companies, as the people list does.
 */
function TagSelected({ companies }: { companies: Company[] }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const shared = (companies[0]?.tags ?? []).filter((tag) =>
    companies.every((company) => company.tags.some((other) => other.id === tag.id)),
  );
  const change = useMutation({
    mutationFn: (changes: Parameters<typeof updateCompany>[2]) =>
      Promise.all(companies.map((company) => updateCompany(workspace, company.id, changes))),
    onSettled: () => client.invalidateQueries({ queryKey: ["workspace", workspace, "companies"] }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <TagPicker
      workspace={workspace}
      size="medium"
      label="Tag the selected companies"
      selected={shared}
      onAdd={(name) => change.mutate({ add_tags: [name] })}
      onRemove={(tag) => change.mutate({ remove_tags: [tag.id] })}
    />
  );
}

const columns: ColumnDef<Company>[] = [
  { id: "name", header: "Company", cell: ({ row }) => <CompanyName company={row.original} /> },
  {
    id: "domain",
    header: "Domain",
    cell: ({ row }) => <Muted ltr>{row.original.domain ?? "—"}</Muted>,
  },
  {
    id: "people",
    header: "People",
    cell: ({ row }) => (
      <span className="text-body-medium text-text-primary">{formatNumber(row.original.people_count ?? 0)}</span>
    ),
  },
  { id: "tags", header: "Tags", cell: ({ row }) => <Tags tags={row.original.tags} /> },
  {
    id: "created",
    header: "Added",
    cell: ({ row }) => <Muted>{formatDate(row.original.created_at)}</Muted>,
  },
];

const widths: Record<string, string> = {
  name: "w-[260px]",
  domain: "w-[260px]",
  people: "w-[120px]",
  tags: "w-[280px]",
  created: "w-[160px]",
};

function totalText(total: number | undefined): string {
  if (total === undefined) {
    return "—";
  }
  const shown = total >= peopleCap ? `${formatNumber(peopleCap)}+` : formatNumber(total);

  return `${shown} ${total === 1 ? "company" : "companies"}`;
}

export function CompaniesPage() {
  const workspace = useWorkspace();
  const navigate = useNavigate();
  const { company: open } = useSearch({ strict: false }) as { company?: string };
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [selection, setSelection] = useState<RowSelectionState>({});
  const [adding, setAdding] = useState(false);
  const q = useDeferredValue(search.trim());
  const list = useQuery({ ...companiesPageQuery(workspace, q, page), enabled: workspace !== "" });
  const companies = list.data?.items ?? [];
  const total = list.data?.total;
  const show = (company: string | undefined) =>
    navigate({ to: ".", search: company ? { company } : {}, replace: company === undefined });

  return (
    <>
      <CustomersActions>
        <ImportButton kind="companies" />
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
          New company
        </Button>
      </CustomersActions>
      <div className="h-full px-4 pb-4">
        <DataTable
          aria-label="Companies"
          data={companies}
          columns={columns}
          getRowId={(company) => company.id}
          total={list.isError ? list.error.message : totalText(total)}
          toolbar={
            <InputBase
              aria-label="Search companies"
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
          }
          selectionBar={(selected, clear) => (
            <>
              <span className="text-body-medium whitespace-nowrap text-text-secondary">
                {formatNumber(selected.length)} selected
              </span>
              <TagSelected companies={companies.filter((company) => selected.includes(company.id))} />
              <LinkButton variant="secondary" onClick={clear}>
                Clear
              </LinkButton>
            </>
          )}
          selection={selection}
          onSelectionChange={setSelection}
          page={page}
          pageCount={Math.ceil(Math.min(total ?? 0, peopleCap) / peoplePerPage)}
          onPageChange={(next) => {
            setPage(next);
            setSelection({});
          }}
          onRowAction={(id) => show(id)}
          widths={widths}
          loading={list.isPending}
          skeleton={<TableSkeleton />}
          empty={q ? "No company matches." : "No companies yet."}
        />
      </div>
      <NewCompanyDialog
        isOpen={adding}
        onClose={(created) => {
          setAdding(false);
          if (created) {
            void show(created);
          }
        }}
      />
      <CompanyDrawer id={open} onClose={() => show(undefined)} />
    </>
  );
}
