/*
 * One customer's page: their details, which the team edits, what the
 * product told dimmi about them and the attributes the team sets, the
 * channels they reach the team on, their tags, the team's notes, the
 * people who look like them, and their conversations as far as the
 * reader may see them. Those who may answer privacy requests export or
 * erase the customer from here.
 */
import { RiCloseLine, RiLinkUnlinkM, RiUserUnfollowLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Textarea } from "@/components/base/textarea/textarea";
import { show, when } from "@/features/inbox/customer/format";
import { Copyable, Nothing, Row } from "@/features/inbox/customer/rows";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { avatarColor } from "@/features/shell/avatar-color";
import { EmptyState } from "@/features/shell/page";
import { ProfileSkeleton } from "@/features/shell/table-skeleton";
import { ApiError } from "@/lib/api/client";
import {
  type Contact,
  type ContactChanges,
  contactConversationsQuery,
  contactQuery,
  detachIdentity,
  duplicatesQuery,
  identitiesQuery,
  updateContact,
} from "@/lib/api/customers";
import { contactEventsQuery } from "@/lib/api/events";
import { callerQuery } from "@/lib/api/workspace";
import { EraseCustomer } from "./erase-customer";
import { ExportCustomer } from "./export-customer";
import { CustomersFrame } from "./frame";
import { personName, useWorkspace } from "./names";
import { RecordConversations } from "./record-conversations";

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

function useWrite(contact: string) {
  const workspace = useWorkspace();
  const client = useQueryClient();

  return useMutation({
    mutationFn: (changes: ContactChanges) => updateContact(workspace, contact, changes),
    onSuccess: () => client.invalidateQueries({ queryKey: ["workspace", workspace] }),
  });
}

function blankToNull(value: string): string | null {
  return value.trim() === "" ? null : value.trim();
}

function Details({ contact }: { contact: Contact }) {
  const write = useWrite(contact.id);
  const [name, setName] = useState(contact.name ?? "");
  const [email, setEmail] = useState(contact.email ?? "");
  const [phone, setPhone] = useState(contact.phone ?? "");
  const fields = write.error instanceof ApiError ? write.error.fields : {};
  const changed = name !== (contact.name ?? "") || email !== (contact.email ?? "") || phone !== (contact.phone ?? "");

  return (
    <Card title="Details">
      <form
        className="flex flex-col gap-3"
        onSubmit={(event) => {
          event.preventDefault();
          write.mutate({ name: blankToNull(name), email: blankToNull(email), phone: blankToNull(phone) });
        }}
      >
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
        <Input
          label="Phone"
          type="tel"
          inputDir="ltr"
          value={phone}
          onChange={setPhone}
          isInvalid={"phone" in fields}
          errorMessage={fields.phone}
        />
        <Button type="submit" size="small" className="self-start" loading={write.isPending} disabled={!changed}>
          Save
        </Button>
      </form>
    </Card>
  );
}

/*
 * attributeValue reads what a teammate typed the way the product sends
 * it: true, false and numbers keep their type, anything else is text.
 */
function attributeValue(text: string): unknown {
  const trimmed = text.trim();
  if (trimmed === "true" || trimmed === "false") {
    return trimmed === "true";
  }

  return trimmed !== "" && Number.isFinite(Number(trimmed)) ? Number(trimmed) : trimmed;
}

function NewAttribute({ contact }: { contact: Contact }) {
  const write = useWrite(contact.id);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const refused = write.error instanceof ApiError ? write.error.message : null;

  return (
    <form
      aria-label="Set an attribute"
      className="flex flex-col gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        write.mutate(
          { custom_attributes: { [key.trim()]: attributeValue(value) } },
          {
            onSuccess: () => {
              setKey("");
              setValue("");
            },
          },
        );
      }}
    >
      <div className="flex items-start gap-2">
        <Input
          aria-label="Attribute name"
          size="small"
          inputDir="ltr"
          placeholder="plan"
          value={key}
          onChange={setKey}
        />
        <Input aria-label="Attribute value" size="small" placeholder="pro" value={value} onChange={setValue} />
        <Button type="submit" variant="secondary" size="small" loading={write.isPending} disabled={key.trim() === ""}>
          Set
        </Button>
      </div>
      {refused ? <p className="text-body-2-regular text-text-error-primary">{refused}</p> : null}
    </form>
  );
}

function Attributes({ contact }: { contact: Contact }) {
  const write = useWrite(contact.id);
  const entries = Object.entries(contact.custom_attributes);

  return (
    <Card title="Attributes">
      <dl className="flex flex-col">
        <Row label="Kind">{contact.kind === "user" ? "User" : "Lead"}</Row>
        {contact.external_id ? (
          <Row label="User id">
            <Copyable value={contact.external_id} label="user id" />
          </Row>
        ) : null}
        {contact.company ? (
          <Row label="Company">
            <Link
              to="/d/customers/companies/$company"
              params={{ company: contact.company.id }}
              className="truncate rounded-md outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              {contact.company.name}
            </Link>
          </Row>
        ) : null}
        <Row label="First seen">{when(contact.first_seen_at ?? contact.created_at)}</Row>
        <Row label="Last seen">{when(contact.last_seen_at)}</Row>
        {entries.map(([key, value]) => (
          <Row key={key} label={<span className="truncate">{key}</span>}>
            <span className="flex min-w-0 items-center gap-1">
              <Copyable value={show(value)} label={key} />
              <IconButton
                size="small"
                icon={RiCloseLine}
                aria-label={`Remove ${key}`}
                onClick={() => write.mutate({ custom_attributes: { [key]: null } })}
              />
            </span>
          </Row>
        ))}
      </dl>
      <NewAttribute contact={contact} />
    </Card>
  );
}

/*
 * Activity is what the person did in the team's product, as its events
 * report: each event's total, its count in the last 7 days and when it
 * last happened.
 */
function Activity({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const { data } = useQuery({ ...contactEventsQuery(workspace, contact.id), enabled: workspace !== "" });
  if (!data || data.length === 0) {
    return null;
  }

  return (
    <Card title="Activity">
      <ul className="flex flex-col gap-2">
        {data.map((e) => (
          <li key={e.name} className="flex min-w-0 flex-col">
            <span className="truncate text-body-2-medium text-text-primary">{e.label}</span>
            <span className="text-body-2-regular text-text-secondary">
              {e.count === 1 ? "Once" : `${e.count} times`}, {e.last_7_days} in the last 7 days, last {when(e.last_at)}
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function Identities({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const query = identitiesQuery(workspace, contact.id);
  const { data: identities = [] } = useQuery(query);
  const detach = useMutation({
    mutationFn: (id: string) => detachIdentity(workspace, contact.id, id),
    onSuccess: () => client.invalidateQueries({ queryKey: query.queryKey }),
  });

  return (
    <Card title="Channels">
      {identities.length === 0 ? <Nothing>No channel knows them yet.</Nothing> : null}
      <ul className="flex flex-col">
        {identities.map((identity) => (
          <li key={identity.id} className="flex min-h-9 items-center gap-2 text-body-2-regular">
            <span className="w-24 shrink-0 text-text-secondary">{identity.type}</span>
            <span dir="auto" className="min-w-0 flex-1 truncate text-text-primary">
              {identity.value}
            </span>
            <IconButton
              size="small"
              icon={RiLinkUnlinkM}
              aria-label={`Detach ${identity.value}`}
              onClick={() => detach.mutate(identity.id)}
            />
          </li>
        ))}
      </ul>
    </Card>
  );
}

function TagsAndNotes({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const write = useWrite(contact.id);
  const [notes, setNotes] = useState(contact.notes ?? "");

  return (
    <Card title="Tags and notes">
      <div className="flex flex-wrap items-center gap-1.5">
        {contact.tags.map((tag) => (
          <TagChip key={tag.id} tag={tag} onRemove={() => write.mutate({ remove_tags: [tag.id] })} />
        ))}
        <TagPicker
          workspace={workspace}
          selected={contact.tags}
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
          const next = blankToNull(notes);
          if (next !== contact.notes) {
            write.mutate({ notes: next });
          }
        }}
      />
    </Card>
  );
}

function Duplicates({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const { data: duplicates = [] } = useQuery(duplicatesQuery(workspace, contact.id));
  if (duplicates.length === 0) {
    return null;
  }

  return (
    <Card title="Possible duplicates">
      <ul className="flex flex-col">
        {duplicates.map((duplicate) => (
          <li key={duplicate.contact.id} className="flex min-h-9 items-center gap-2 text-body-2-regular">
            <Link
              to="/d/customers/$contact"
              params={{ contact: duplicate.contact.id }}
              className="min-w-0 flex-1 truncate rounded-md text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              {personName(duplicate.contact)}
            </Link>
            <span className="text-text-tertiary">{duplicate.reasons.map(reasonText).join(", ")}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function reasonText(reason: string): string {
  const text: Record<string, string> = {
    same_email: "same email",
    same_phone: "same phone",
    same_whatsapp: "same WhatsApp number",
    same_name_company: "same name and company",
  };

  return text[reason] ?? reason;
}

export function ContactProfile({ id }: { id: string }) {
  const workspace = useWorkspace();
  const { data: contact, isPending } = useQuery({ ...contactQuery(workspace, id), retry: false });
  const conversations = useQuery(contactConversationsQuery(workspace, id, 20));
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });

  if (!contact) {
    return (
      <CustomersFrame tabs={false}>
        {isPending ? (
          <ProfileSkeleton />
        ) : (
          <EmptyState
            icon={RiUserUnfollowLine}
            title="Customer not found"
            description="They may have been removed, or they belong to another workspace."
          />
        )}
      </CustomersFrame>
    );
  }

  const name = personName(contact);
  const canErase = caller?.member?.permissions.includes("contacts:erase") ?? false;

  return (
    <CustomersFrame
      tabs={false}
      actions={
        canErase ? (
          <div className="flex items-center gap-2">
            <ExportCustomer contact={contact} />
            <EraseCustomer contact={contact} />
          </div>
        ) : null
      }
      title={
        <span className="flex items-center gap-2">
          <Avatar size="xs" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />
          {name}
        </span>
      }
    >
      <div className="grid gap-3 px-4 pb-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div className="flex flex-col gap-3">
          <Details key={contact.updated_at} contact={contact} />
          <Attributes contact={contact} />
          <Activity contact={contact} />
          <Identities contact={contact} />
        </div>
        <div className="flex flex-col gap-3">
          <TagsAndNotes key={contact.id} contact={contact} />
          <Duplicates contact={contact} />
          <Card title="Conversations">
            <RecordConversations conversations={conversations.data ?? []} />
          </Card>
        </div>
      </div>
    </CustomersFrame>
  );
}
