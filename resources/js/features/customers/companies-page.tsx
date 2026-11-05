/*
 * The companies the workspace's customers work at, searched by name or
 * domain, and a company's own page: its details, tags and notes, the
 * people who work there and their conversations.
 */
import { RiAddLine, RiBuilding2Line, RiSearchLine } from "@remixicon/react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useDeferredValue, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { Textarea } from "@/components/base/textarea/textarea";
import { show } from "@/features/inbox/customer/format";
import { Copyable, Nothing, Row } from "@/features/inbox/customer/rows";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { EmptyState } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";
import {
  type Company,
  companiesQuery,
  companyConversationsQuery,
  companyPeopleQuery,
  companyQuery,
  createCompany,
  updateCompany,
} from "@/lib/api/customers";

import { CustomersFrame } from "./frame";
import { personName, useWorkspace } from "./names";
import { RecordConversations } from "./record-conversations";

function NewCompanyDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");
  const create = useMutation({
    mutationFn: () => createCompany(workspace, { name: name.trim(), domain: domain.trim() || undefined }),
    onSuccess: async (company) => {
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "companies"] });
      await navigate({ to: "/d/customers/companies/$company", params: { company: company.id } });
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

export function CompaniesPage() {
  const workspace = useWorkspace();
  const [search, setSearch] = useState("");
  const [adding, setAdding] = useState(false);
  const q = useDeferredValue(search.trim());
  const list = useInfiniteQuery({ ...companiesQuery(workspace, q), enabled: workspace !== "" });
  const companies = list.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <CustomersFrame
      actions={
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
          New company
        </Button>
      }
    >
      <div className="flex flex-col gap-3 px-4 pb-4">
        <Input
          aria-label="Search companies"
          size="small"
          className="w-64"
          leadingIcon={RiSearchLine}
          placeholder="Search by name or domain"
          value={search}
          onChange={setSearch}
        />
        <Table aria-label="Companies" size="sm" className="[&_th]:border-t-0">
          <TableHeader>
            <TableColumn isRowHeader>Name</TableColumn>
            <TableColumn>Domain</TableColumn>
            <TableColumn>Tags</TableColumn>
          </TableHeader>
          <TableBody
            items={companies}
            renderEmptyState={() => <p className="py-2 text-body-2-regular text-text-tertiary">No companies yet.</p>}
          >
            {(company) => (
              <TableRow id={company.id}>
                <TableCell>
                  <Link
                    to="/d/customers/companies/$company"
                    params={{ company: company.id }}
                    className="rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                  >
                    {company.name}
                  </Link>
                </TableCell>
                <TableCell>
                  <span dir="ltr" className="text-body-2-regular text-text-secondary">
                    {company.domain ?? "—"}
                  </span>
                </TableCell>
                <TableCell>
                  <span className="flex flex-wrap gap-1">
                    {company.tags.map((tag) => (
                      <TagChip key={tag.id} tag={tag} />
                    ))}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        {list.hasNextPage ? (
          <Button variant="secondary" size="small" className="self-center" onClick={() => list.fetchNextPage()}>
            Load more
          </Button>
        ) : null}
      </div>
      <NewCompanyDialog isOpen={adding} onClose={() => setAdding(false)} />
    </CustomersFrame>
  );
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section
      aria-label={title}
      className="flex flex-col gap-2 rounded-3xl border border-border-button-default bg-background-primary-default p-4"
    >
      <h2 className="text-body-medium text-text-primary">{title}</h2>
      {children}
    </section>
  );
}

function CompanyDetails({ company }: { company: Company }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const write = useMutation({
    mutationFn: (changes: Parameters<typeof updateCompany>[2]) => updateCompany(workspace, company.id, changes),
    onSuccess: () => client.invalidateQueries({ queryKey: ["workspace", workspace] }),
  });
  const [notes, setNotes] = useState(company.notes ?? "");

  return (
    <Card title="Details">
      <dl className="flex flex-col">
        <Row label="Domain">{company.domain ? <Copyable value={company.domain} label="domain" /> : "—"}</Row>
        {company.external_id ? (
          <Row label="Company id">
            <Copyable value={company.external_id} label="company id" />
          </Row>
        ) : null}
        {Object.entries(company.custom_attributes).map(([key, value]) => (
          <Row key={key} label={<span className="truncate">{key}</span>}>
            <Copyable value={show(value)} label={key} />
          </Row>
        ))}
      </dl>
      <div className="flex flex-wrap items-center gap-1.5">
        {company.tags.map((tag) => (
          <TagChip key={tag.id} tag={tag} onRemove={() => write.mutate({ remove_tags: [tag.id] })} />
        ))}
        <TagPicker
          workspace={workspace}
          selected={company.tags}
          onAdd={(tag) => write.mutate({ add_tags: [tag] })}
          onRemove={(tag) => write.mutate({ remove_tags: [tag.id] })}
        />
      </div>
      <Textarea
        aria-label="Notes"
        placeholder="Add a note only the team sees"
        rows={3}
        autoResize
        maxRows={12}
        maxLength={10000}
        value={notes}
        onChange={setNotes}
        onBlur={() => {
          const next = notes.trim() === "" ? null : notes;
          if (next !== company.notes) {
            write.mutate({ notes: next });
          }
        }}
      />
    </Card>
  );
}

export function CompanyPage({ id }: { id: string }) {
  const workspace = useWorkspace();
  const { data: company, isPending } = useQuery({ ...companyQuery(workspace, id), retry: false });
  const { data: people = [] } = useQuery(companyPeopleQuery(workspace, id));
  const { data: conversations = [] } = useQuery(companyConversationsQuery(workspace, id));

  if (!company) {
    return (
      <CustomersFrame tabs={false}>
        {isPending ? null : (
          <EmptyState
            icon={RiBuilding2Line}
            title="Company not found"
            description="It may have been deleted, or it belongs to another workspace."
          />
        )}
      </CustomersFrame>
    );
  }

  return (
    <CustomersFrame tabs={false} title={company.name}>
      <div className="grid gap-3 px-4 pb-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <CompanyDetails key={company.id} company={company} />
        <div className="flex flex-col gap-3">
          <Card title="People">
            {people.length === 0 ? <Nothing>No one works here yet.</Nothing> : null}
            <ul className="flex flex-col">
              {people.map((person) => (
                <li key={person.id} className="flex min-h-9 items-center">
                  <Link
                    to="/d/customers/$contact"
                    params={{ contact: person.id }}
                    className="truncate rounded-md text-body-2-regular text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                  >
                    {personName(person)}
                  </Link>
                </li>
              ))}
            </ul>
          </Card>
          <Card title="Conversations">
            <RecordConversations conversations={conversations} />
          </Card>
        </div>
      </div>
    </CustomersFrame>
  );
}
