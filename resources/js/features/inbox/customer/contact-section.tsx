/*
 * Who the customer is: their name, which a teammate edits in place, how
 * to reach them, what the product told dimmi about them, where this
 * conversation started, their tags and the team's private notes on them.
 * Notes save when the field loses focus, and never reach the customer.
 */
import { RiCheckboxCircleFill, RiPencilLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Textarea } from "@/components/base/textarea/textarea";
import { avatarColor } from "@/features/shell/avatar-color";
import { type ContactChanges, type ConversationContext, updateContact } from "@/lib/api/customers";

import { TagChip } from "../tags/tag-chip";
import { TagPicker } from "../tags/tag-picker";
import { AttributeRows } from "./attribute-rows";
import { when } from "./format";
import { Copyable, Nothing, Row } from "./rows";

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

function Name({ workspace, context }: { workspace: string; context: ConversationContext }) {
  const contact = context.contact;
  const name = contact.name ?? contact.email ?? "Unknown customer";
  const write = useContactWrite(workspace, contact.id);
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(contact.name ?? "");

  if (editing) {
    return (
      <Input
        aria-label="Customer name"
        size="small"
        autoFocus
        value={text}
        onChange={setText}
        onBlur={() => setEditing(false)}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            setEditing(false);
          }
          if (event.key === "Enter") {
            write.mutate({ name: text.trim() === "" ? null : text.trim() });
            setEditing(false);
          }
        }}
      />
    );
  }

  return (
    <div className="flex min-w-0 items-center gap-1">
      <Link
        to="/d/customers/$contact"
        params={{ contact: contact.id }}
        className="truncate rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {name}
      </Link>
      <IconButton
        size="small"
        icon={RiPencilLine}
        aria-label="Edit name"
        onClick={() => {
          setText(contact.name ?? "");
          setEditing(true);
        }}
      />
    </div>
  );
}

function Notes({ workspace, context }: { workspace: string; context: ConversationContext }) {
  const contact = context.contact;
  const write = useContactWrite(workspace, contact.id);
  const [text, setText] = useState(contact.notes ?? "");

  return (
    <Textarea
      aria-label="Notes"
      placeholder="Add a note only the team sees"
      rows={2}
      autoResize
      maxRows={8}
      maxLength={10000}
      value={text}
      onChange={setText}
      onBlur={() => {
        const next = text.trim() === "" ? null : text;
        if (next !== contact.notes) {
          write.mutate({ notes: next });
        }
      }}
    />
  );
}

function Origin({ context }: { context: ConversationContext }) {
  const source = context.source;
  const rows: [string, string | undefined][] = [
    ["Page", source.page_url],
    ["Browser", source.browser],
    ["OS", source.os],
    ["Device", source.device],
    ["App version", source.app_version],
  ];

  return (
    <>
      {rows.map(([label, value]) =>
        value ? (
          <Row key={label} label={label}>
            <Copyable value={value} label={label.toLowerCase()} />
          </Row>
        ) : null,
      )}
    </>
  );
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

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2.5">
        <Avatar size="md" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />
        <div className="flex min-w-0 flex-1 flex-col">
          <Name key={contact.id} workspace={workspace} context={context} />
          <span className="truncate text-body-2-regular text-text-tertiary">
            {contact.kind === "user" ? "User" : "Lead"}
            {contact.company ? ` at ${contact.company.name}` : ""}
          </span>
        </div>
      </div>
      <dl className="flex flex-col">
        <AttributeRows workspace={workspace} context={context} pinnedOnly />
        <Row label="Email">
          {contact.email ? <Copyable value={contact.email} label="email" /> : "—"}
          {contact.email && contact.email_verified ? (
            <RiCheckboxCircleFill aria-label="Verified" className="size-4 shrink-0 text-foreground-icon-secondary" />
          ) : null}
        </Row>
        {contact.phone ? (
          <Row label="Phone">
            <Copyable value={contact.phone} label="phone" />
          </Row>
        ) : null}
        {contact.external_id ? (
          <Row label="User id">
            <Copyable value={contact.external_id} label="user id" />
          </Row>
        ) : null}
        {where ? <Row label="Location">{where}</Row> : null}
        {time ? <Row label="Local time">{time}</Row> : null}
        <AttributeRows workspace={workspace} context={context} pinnedOnly={false} />
        <Row label="First seen">{when(contact.first_seen_at ?? contact.created_at)}</Row>
        <Row label="Last seen">{when(contact.last_seen_at)}</Row>
        <Origin context={context} />
      </dl>
      <div className="flex flex-col gap-1">
        <span className="text-body-2-regular text-text-secondary">Channels</span>
        {context.identities.length === 0 ? <Nothing>No channel knows them yet.</Nothing> : null}
        <ul className="flex flex-col">
          {context.identities.map((identity) => (
            <li key={identity.id} className="flex min-h-8 items-center justify-between gap-2 text-body-2-regular">
              <span className="text-text-secondary">{identity.type}</span>
              <span dir="auto" className="min-w-0 truncate text-text-primary">
                {identity.value}
              </span>
            </li>
          ))}
        </ul>
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        {contact.tags.map((tag) => (
          <TagChip key={tag.id} tag={tag} onRemove={() => write.mutate({ remove_tags: [tag.id] })} />
        ))}
        <TagPicker
          workspace={workspace}
          label="Add tag to customer"
          selected={contact.tags}
          onAdd={(tag) => write.mutate({ add_tags: [tag] })}
          onRemove={(tag) => write.mutate({ remove_tags: [tag.id] })}
        />
      </div>
      <Notes key={contact.id} workspace={workspace} context={context} />
    </div>
  );
}
