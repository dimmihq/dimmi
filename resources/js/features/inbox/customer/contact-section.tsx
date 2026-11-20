/*
 * Who the customer is: their name, with a button that opens the
 * customer's edit modal, how
 * to reach them, what the product told dimmi about them, where this
 * conversation started, the channels that know them and their tags.
 * Pinned attributes come first and only the first five rows show until
 * the teammate asks for the rest. Every value copies when pressed. The team's private notes follow as
 * their own section: text until someone writes in it, saved when the
 * field loses focus, and never shown to the customer.
 */
import { RiCheckboxCircleFill, RiPencilLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { ContactEditModal } from "@/features/customers/contact-edit";
import { avatarColor } from "@/features/shell/avatar-color";
import { type ContactChanges, type ConversationContext, updateContact } from "@/lib/api/customers";

import { TagChip } from "../tags/tag-chip";
import { TagPicker } from "../tags/tag-picker";
import { AttributeRow } from "./attribute-rows";
import { CustomerNotes } from "./customer-notes";
import { when } from "./format";
import { Copyable, Row, Rows, Section } from "./rows";
import { usePins } from "./use-pins";

/*
 * useContactWrite sends a change to the contact and refreshes what shows
 * it: this panel and the conversations naming the customer.
 */
function useContactWrite(workspace: string, contact: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (changes: ContactChanges) => updateContact(workspace, contact, changes),
    onSuccess: () => client.invalidateQueries({ queryKey: ["workspace", workspace] }),
  });
}

/*
 * Name is the customer's name, which opens their page, with their
 * company beside it and a button that opens the same edit modal the
 * customers list uses.
 */
function Name({ context }: { context: ConversationContext }) {
  const contact = context.contact;
  const [editing, setEditing] = useState(false);

  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <Link
        to="/d/customers/$contact"
        params={{ contact: contact.id }}
        className="truncate rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {contact.name ?? contact.email ?? "Unknown customer"}
      </Link>
      {contact.company ? (
        <span className="min-w-0 truncate text-body-2-regular text-text-tertiary">{contact.company.name}</span>
      ) : null}
      <Button
        variant="secondary"
        size="xs"
        iconOnly
        leadingIcon={RiPencilLine}
        aria-label="Edit customer"
        className="shrink-0 opacity-0 transition-opacity duration-150 focus-visible:opacity-100 group-hover/who:opacity-100"
        onClick={() => setEditing(true)}
      />
      {editing ? <ContactEditModal id={contact.id} onClose={() => setEditing(false)} /> : null}
    </div>
  );
}

/*
 * origin is where the conversation started, as label and value pairs.
 */
function origin(context: ConversationContext): [string, string][] {
  const source = context.source;
  const rows: [string, string | undefined][] = [
    ["Page", source.page_url],
    ["Browser", source.browser],
    ["OS", source.os],
    ["Device", source.device],
    ["App version", source.app_version],
  ];

  return rows.filter((row): row is [string, string] => Boolean(row[1]));
}

function localTime(timezone: string | null): string | null {
  if (!timezone) {
    return null;
  }

  try {
    return new Date().toLocaleTimeString(undefined, { timeZone: timezone, hour: "numeric", minute: "2-digit" });
  } catch {
    return null;
  }
}

function place(context: ConversationContext): string | null {
  const location = context.contact.location;
  const parts = [location?.city, location?.region, location?.country].filter(Boolean);

  return parts.length > 0 ? parts.join(", ") : null;
}

export function ContactSection({ workspace, context }: { workspace: string; context: ConversationContext }) {
  const contact = context.contact;
  const name = contact.name ?? contact.email ?? "Unknown customer";
  const write = useContactWrite(workspace, contact.id);
  const time = localTime(contact.timezone);
  const where = place(context);
  const { pinned, toggle } = usePins(workspace);
  const attribute = ([key, value]: [string, unknown]) => (
    <AttributeRow
      key={`attribute:${key}`}
      name={key}
      value={value}
      pinned={pinned.includes(key)}
      onToggle={() => toggle(key)}
    />
  );
  const entries = Object.entries(contact.custom_attributes);
  const copy = (label: string, value: string) => (
    <Row key={label} label={<span className="truncate">{label}</span>}>
      <Copyable value={value} label={label.toLowerCase()} />
    </Row>
  );
  const plain = (label: string, value: string) => (
    <Row key={label} label={label}>
      <span className="truncate">{value}</span>
    </Row>
  );
  const rows = [
    ...entries
      .filter(([key]) => pinned.includes(key))
      .sort(([a], [b]) => pinned.indexOf(a) - pinned.indexOf(b))
      .map(attribute),
    <Row key="Email" label="Email">
      {contact.email ? <Copyable value={contact.email} label="email" /> : "—"}
      {contact.email && contact.email_verified ? (
        <RiCheckboxCircleFill aria-label="Verified" className="size-3.5 shrink-0 text-foreground-icon-tertiary" />
      ) : null}
    </Row>,
    contact.phone ? copy("Phone", contact.phone) : null,
    contact.external_id ? copy("User id", contact.external_id) : null,
    ...entries.filter(([key]) => !pinned.includes(key)).map(attribute),
    where ? plain("Location", where) : null,
    time ? plain("Local time", time) : null,
    plain("First seen", when(contact.first_seen_at ?? contact.created_at)),
    plain("Last seen", when(contact.last_seen_at)),
    ...origin(context).map(([label, value]) => copy(label, value)),
    ...context.identities
      .filter((identity) => ![contact.email, contact.phone].includes(identity.value))
      .filter((identity) => !platformIdentities.includes(identity.type))
      .map((identity) => (
        <Row key={identity.id} label={<span className="truncate">{identityLabel(identity.type)}</span>}>
          <Copyable value={identity.value} label={identityLabel(identity.type)} />
        </Row>
      )),
  ].filter(Boolean);

  return (
    <>
      <Section title="Customer">
        <div className="group/who flex items-center gap-2.5 pb-1">
          <Avatar size="sm" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />
          <div className="min-w-0 flex-1">
            <Name key={contact.id} context={context} />
          </div>
        </div>
        <dl className="flex flex-col">
          <Row label="Tags">
            <div className="flex min-w-0 flex-wrap items-center gap-1 py-0.5">
              {contact.tags.map((tag) => (
                <TagChip key={tag.id} tag={tag} size="small" onRemove={() => write.mutate({ remove_tags: [tag.id] })} />
              ))}
              <TagPicker
                workspace={workspace}
                label="Add tag to customer"
                selected={contact.tags}
                onAdd={(tag) => write.mutate({ add_tags: [tag] })}
                onRemove={(tag) => write.mutate({ remove_tags: [tag.id] })}
                quiet
              />
            </div>
          </Row>
        </dl>
        <Rows key={contact.id} rows={rows} />
      </Section>
      <Section title="Customer notes">
        <CustomerNotes key={contact.id} workspace={workspace} about={{ kind: "contact", id: contact.id }} />
      </Section>
    </>
  );
}

/*
 * platformIdentities are ids only Meta's servers can use, a number that
 * tells a teammate nothing; the customer's username is shown in their
 * place, in the platform's own section.
 */
const platformIdentities = ["facebook_user", "instagram_user"];

/*
 * identityLabels name the identities channels know a customer by; any
 * other type shows its own name, spaced out and capitalised.
 */
const identityLabels: Record<string, string> = {
  whatsapp_user: "WhatsApp",
  facebook_user: "Messenger",
  instagram_user: "Instagram",
  slack_user: "Slack",
  external_id: "External ID",
  widget_visitor: "Visitor",
};

function identityLabel(type: string): string {
  const spaced = type.replace(/_/g, " ");
  return identityLabels[type] ?? spaced.charAt(0).toUpperCase() + spaced.slice(1);
}
