/*
 * One company, in a drawer beside the list: its details, tags and notes,
 * the people who work there and their conversations.
 */
import { RiBuilding2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Drawer, DrawerBody, DrawerHeader } from "@/components/base/drawer/drawer";
import { Textarea } from "@/components/base/textarea/textarea";
import { show } from "@/features/inbox/customer/format";
import { Copyable, Nothing, Row } from "@/features/inbox/customer/rows";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { avatarColor } from "@/features/shell/avatar-color";
import { EmptyState } from "@/features/shell/page";
import { ListSkeleton, ProfileSkeleton, TitleSkeleton } from "@/features/shell/table-skeleton";
import {
  type Company,
  companyConversationsQuery,
  companyPeopleQuery,
  companyQuery,
  updateCompany,
} from "@/lib/api/customers";

import { personName, useWorkspace } from "./names";
import { RecordConversations } from "./record-conversations";

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section
      aria-label={title}
      className="flex flex-col gap-2 rounded-3xl border border-border-button-default bg-background-secondary-default p-4"
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

function CompanyPanel({ id }: { id: string }) {
  const workspace = useWorkspace();
  const { data: company, isPending } = useQuery({ ...companyQuery(workspace, id), retry: false });
  const { data: people = [], isPending: peopleLoading } = useQuery(companyPeopleQuery(workspace, id));
  const conversations = useQuery(companyConversationsQuery(workspace, id));

  if (!company) {
    return (
      <>
        <DrawerHeader title={isPending ? <TitleSkeleton /> : "Company not found"} />
        <DrawerBody>
          {isPending ? (
            <ProfileSkeleton />
          ) : (
            <EmptyState
              icon={RiBuilding2Line}
              title="Company not found"
              description="It may have been deleted, or it belongs to another workspace."
            />
          )}
        </DrawerBody>
      </>
    );
  }

  return (
    <>
      <DrawerHeader
        title={
          <span className="flex min-w-0 items-center gap-2.5">
            <Avatar
              size="md"
              color={avatarColor(company.name)}
              initials={company.name.charAt(0).toUpperCase()}
              className="rounded-lg"
            />
            <span className="flex min-w-0 flex-col">
              <span className="truncate">{company.name}</span>
              {company.domain ? (
                <span dir="ltr" className="truncate text-body-2-regular text-text-tertiary">
                  {company.domain}
                </span>
              ) : null}
            </span>
          </span>
        }
      />
      <DrawerBody className="pb-6">
        <div className="flex flex-col gap-3 pt-2">
          <CompanyDetails key={company.id} company={company} />
          <Card title="People">
            {peopleLoading ? <ListSkeleton /> : null}
            {!peopleLoading && people.length === 0 ? <Nothing>No one works here yet.</Nothing> : null}
            <ul className="flex flex-col">
              {people.map((person) => (
                <li key={person.id} className="flex min-h-9 items-center">
                  <Link
                    to="/d/customers"
                    search={{ contact: person.id }}
                    className="truncate rounded-md text-body-2-regular text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                  >
                    {personName(person)}
                  </Link>
                </li>
              ))}
            </ul>
          </Card>
          <Card title="Conversations">
            <RecordConversations conversations={conversations.data ?? []} loading={conversations.isPending} />
          </Card>
        </div>
      </DrawerBody>
    </>
  );
}

/*
 * CompanyDrawer slides a company in from the end of the page, over the
 * list it was opened from.
 */
export function CompanyDrawer({ id, onClose }: { id: string | undefined; onClose: () => void }) {
  return (
    <Drawer
      isOpen={id !== undefined}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      placement="end"
      size="lg"
      aria-label="Company"
    >
      {id ? <CompanyPanel key={id} id={id} /> : null}
    </Drawer>
  );
}
