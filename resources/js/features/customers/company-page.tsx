/*
 * One company on a page of its own, laid out the way a ticket or a
 * feature request is: the notes the team keeps about it, the people who
 * work there in a table and their conversations in the middle, and
 * beside them, in a panel that resizes, what the company is (its
 * domain, id, attributes and tags). On a narrow screen, where there is
 * no room for the panel, those sit above the notes.
 */
import { RiArrowLeftLine, RiBuilding2Line, RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Avatar } from "@/components/base/avatar/avatar";
import { InputBase } from "@/components/base/input/input";
import { CustomerNotes } from "@/features/inbox/customer/customer-notes";
import { show } from "@/features/inbox/customer/format";
import { Copyable, Row, Rows } from "@/features/inbox/customer/rows";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { avatarColor } from "@/features/shell/avatar-color";
import { EmptyState, Panel, ScrollArea } from "@/features/shell/page";
import { ProfileSkeleton, TableSkeleton } from "@/features/shell/table-skeleton";
import { useWide } from "@/features/shell/use-wide";
import { DetailsColumn, TicketSplit } from "@/features/tickets/ticket-thread";
import {
  type Company,
  companyConversationsQuery,
  companyPeopleQuery,
  companyQuery,
  updateCompany,
} from "@/lib/api/customers";
import { countOf } from "@/lib/i18n/format";

import { useWorkspace } from "./names";
import { usePeopleColumns } from "./people-columns";
import { peopleWidths } from "./people-params";
import { RecordConversations } from "./record-conversations";

/*
 * Block is one titled part of the middle column, with how many things
 * it lists.
 */
function Block({ title, count, children }: { title: string; count?: number; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h2 className="flex items-center gap-1.5 text-body-medium text-text-primary">
        {title}
        {count === undefined ? null : <span className="text-text-tertiary tabular-nums">{count}</span>}
      </h2>
      {children}
    </section>
  );
}

/*
 * People lists who works at the company in the people table itself: the
 * same columns the teammate chose there, drawn the same way, less the
 * company, which is this one. It is searched by name or email, and a row
 * opens the person.
 */
function People({ workspace, id }: { workspace: string; id: string }) {
  const navigate = useNavigate();
  const { data: people = [], isPending } = useQuery(companyPeopleQuery(workspace, id));
  const { columns: chosen } = usePeopleColumns();
  const columns = chosen.filter((c) => c.id !== "company").map((c) => ({ ...c, enableSorting: false }));
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const rows = people.filter((p) => needle === "" || `${p.name ?? ""} ${p.email ?? ""}`.toLowerCase().includes(needle));

  return (
    <DataTable
      aria-label="People"
      data={rows}
      columns={columns}
      getRowId={(person) => person.id}
      total={countOf(rows.length, "person", "people")}
      toolbar={
        <InputBase
          aria-label="Search people"
          placeholder="Search"
          leadingIcon={RiSearchLine}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          fieldClassName="min-w-[153px] flex-1 rounded-full bg-background-secondary-default sm:w-[200px] sm:min-w-0 sm:flex-none"
          className="text-body-medium"
        />
      }
      page={1}
      pageCount={1}
      onPageChange={() => undefined}
      onRowAction={(person) => void navigate({ to: "/d/customers", search: { contact: person } })}
      loading={isPending}
      skeleton={<TableSkeleton />}
      widths={peopleWidths}
      empty={needle ? "No one matches." : "No one works here yet."}
    />
  );
}

/*
 * Details is the column beside the company: what it is as rows, and
 * its tags.
 */
function Details({ workspace, company }: { workspace: string; company: Company }) {
  const client = useQueryClient();
  const write = useMutation({
    mutationFn: (changes: Parameters<typeof updateCompany>[2]) => updateCompany(workspace, company.id, changes),
    onSuccess: () => client.invalidateQueries({ queryKey: ["workspace", workspace] }),
  });
  const attributes = Object.entries(company.custom_attributes).map(([key, value]) => (
    <Row key={key} label={<span className="truncate">{key}</span>}>
      <Copyable value={show(value)} label={key} />
    </Row>
  ));

  return (
    <div className="pt-1">
      <div className="flex flex-col">
        <dl className="flex flex-col">
          <Row label="Domain">{company.domain ? <Copyable value={company.domain} label="domain" /> : "—"}</Row>
          {company.external_id ? (
            <Row label="Company id">
              <Copyable value={company.external_id} label="company id" />
            </Row>
          ) : null}
          <Row label="Tags">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5 py-1">
              {company.tags.map((tag) => (
                <TagChip key={tag.id} size="small" tag={tag} onRemove={() => write.mutate({ remove_tags: [tag.id] })} />
              ))}
              <TagPicker
                workspace={workspace}
                label="Add tag to company"
                selected={company.tags}
                onAdd={(tag) => write.mutate({ add_tags: [tag] })}
                onRemove={(tag) => write.mutate({ remove_tags: [tag.id] })}
                quiet
              />
            </div>
          </Row>
        </dl>
        <Rows key={company.id} rows={attributes} most={8} />
      </div>
    </div>
  );
}

function BackLink() {
  return (
    <Link
      to="/d/customers/companies"
      aria-label="Back to companies"
      className="grid size-7 shrink-0 place-items-center rounded-lg text-foreground-icon-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <RiArrowLeftLine aria-hidden className="size-4 rtl:-scale-x-100" />
    </Link>
  );
}

export function CompanyPage({ id }: { id: string }) {
  const workspace = useWorkspace();
  const { data: company, isPending } = useQuery({ ...companyQuery(workspace, id), retry: false });
  const conversations = useQuery(companyConversationsQuery(workspace, id));
  const wide = useWide(1024);

  if (!company) {
    return (
      <Panel label="Company" className="flex h-full w-full flex-1 flex-col">
        <header className="flex h-12 shrink-0 items-center gap-2 ps-2 pe-2.5 pt-[7px]">
          <BackLink />
        </header>
        <ScrollArea className="flex-1 px-4">
          {isPending ? (
            <ProfileSkeleton />
          ) : (
            <EmptyState
              icon={RiBuilding2Line}
              title="Company not found"
              description="It may have been deleted, or it belongs to another workspace."
            />
          )}
        </ScrollArea>
      </Panel>
    );
  }

  return (
    <TicketSplit
      thread={
        <Panel label={company.name} ground className="flex h-full min-w-0 flex-1 flex-col">
          <header className="flex h-12 shrink-0 items-center gap-2 ps-2 pe-2.5 pt-[7px]">
            <BackLink />
            <span className="min-w-0 truncate text-body-medium text-text-primary">{company.name}</span>
          </header>
          <ScrollArea ground className="flex-1">
            <div className="mx-auto flex w-full max-w-[960px] flex-col gap-8 px-4 py-5">
              <div className="flex min-w-0 items-center gap-3">
                <Avatar
                  size="lg"
                  color={avatarColor(company.name)}
                  initials={company.name.charAt(0).toUpperCase()}
                  className="rounded-xl"
                />
                <div className="flex min-w-0 flex-col">
                  <h1 className="truncate text-title-2-medium tracking-tight text-text-primary">{company.name}</h1>
                  <span className="truncate text-body-2-regular text-text-tertiary">
                    {[
                      company.domain,
                      conversations.data ? countOf(conversations.data.length, "conversation", "conversations") : null,
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </span>
                </div>
              </div>
              {wide ? null : <Details workspace={workspace} company={company} />}
              <Block title="Notes">
                <div className="rounded-2xl bg-background-secondary-default p-3">
                  <CustomerNotes
                    key={company.id}
                    workspace={workspace}
                    about={{ kind: "company", id: company.id }}
                    composer
                  />
                </div>
              </Block>
              <People workspace={workspace} id={id} />
              <Block title="Conversations" count={conversations.data?.length}>
                <RecordConversations conversations={conversations.data ?? []} loading={conversations.isPending} />
              </Block>
            </div>
          </ScrollArea>
        </Panel>
      }
      details={
        <DetailsColumn>
          <Details workspace={workspace} company={company} />
        </DetailsColumn>
      }
    />
  );
}
