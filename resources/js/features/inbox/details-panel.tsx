/*
 * What a teammate needs beside the thread, and the moves they make on the
 * case: assign it, change its status or priority, mark it done or snooze
 * it. Below, who the customer is and what is related. Sections fold in
 * BoardUI's divided accordion, straight on the panel, so nothing sits in
 * a card within the card. The sections without data yet say what will
 * appear there.
 */
import {
  RiArticleLine,
  RiCheckLine,
  RiCloseLine,
  RiLinksLine,
  RiTimeLine,
  RiUser3Line,
  RiUserSharedLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import type { Key, ReactNode } from "react";

import { Accordion, AccordionItem } from "@/components/base/accordion/accordion";
import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Select, SelectItem } from "@/components/base/select/select";
import { avatarColor } from "@/features/shell/avatar-color";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";
import type { Conversation, ConversationStatus } from "@/lib/api/inbox";
import { callerQuery, membersQuery } from "@/lib/api/workspace";

import { type CaseControls, type Priority, priorityLabel, statusLabel } from "./case";
import { customerName } from "./conversations";

const nobody = "nobody";
const statusDot = { open: "green", snoozed: "yellow", closed: "indigo" } as const;

/*
 * The sections' keys, which the accordion uses to remember which are open.
 */
const section = { customer: "customer", links: "links", articles: "articles" } as const;

interface Person {
  id: string;
  name: string;
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid min-h-9 grid-cols-[80px_minmax(0,1fr)] items-center gap-2">
      <dt className="text-body-2-regular text-text-secondary">{label}</dt>
      <dd className="flex min-w-0 justify-end text-body-2-medium text-text-primary">{children}</dd>
    </div>
  );
}

function PersonName({ name }: { name: string }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <Avatar size="xs" color={avatarColor(name)} initials={name.charAt(0)} />
      <span className="truncate">{name}</span>
    </span>
  );
}

function Nothing({ children }: { children: ReactNode }) {
  return <p className="text-body-2-regular text-text-tertiary">{children}</p>;
}

function choose<T extends string>(apply: (value: T) => void) {
  return (key: Key | null) => {
    if (key !== null) {
      apply(String(key) as T);
    }
  };
}

function CaseFields({
  conversation,
  caze,
  people,
}: {
  conversation: Conversation;
  caze: CaseControls;
  people: Person[];
}) {
  const priority: Priority = conversation.priority ? "priority" : "normal";

  return (
    <dl className="flex flex-col">
      <Row label="Assignee">
        <Select
          aria-label="Assignee"
          size="sm"
          selectedKey={conversation.assignee?.id ?? nobody}
          onSelectionChange={choose((key) => caze.assign(key === nobody ? null : key))}
        >
          <SelectItem id={nobody} textValue="Unassigned">
            Unassigned
          </SelectItem>
          {people.map((person) => (
            <SelectItem key={person.id} id={person.id} textValue={person.name}>
              <PersonName name={person.name} />
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Status">
        <Select
          aria-label="Status"
          size="sm"
          selectedKey={conversation.status}
          onSelectionChange={choose<ConversationStatus>(caze.setStatus)}
        >
          {(Object.keys(statusLabel) as ConversationStatus[]).map((status) => (
            <SelectItem key={status} id={status} textValue={statusLabel[status]}>
              <span className="flex items-center gap-2">
                <StatusDot color={statusDot[status]} />
                {statusLabel[status]}
              </span>
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Priority">
        <Select
          aria-label="Priority"
          size="sm"
          selectedKey={priority}
          onSelectionChange={choose<Priority>(caze.setPriority)}
        >
          {(Object.keys(priorityLabel) as Priority[]).map((value) => (
            <SelectItem key={value} id={value} textValue={priorityLabel[value]}>
              {priorityLabel[value]}
            </SelectItem>
          ))}
        </Select>
      </Row>
    </dl>
  );
}

function Actions({ conversation, caze, me }: { conversation: Conversation; caze: CaseControls; me: string | null }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {me === null || conversation.assignee?.id === me ? null : (
        <Button variant="secondary" size="small" leadingIcon={RiUserSharedLine} onClick={() => caze.assign(me)}>
          Assign to me
        </Button>
      )}
      {conversation.status === "open" ? (
        <Button variant="secondary" size="small" leadingIcon={RiTimeLine} onClick={() => caze.setStatus("snoozed")}>
          Snooze
        </Button>
      ) : null}
      {conversation.status === "closed" ? (
        <Button variant="secondary" size="small" onClick={() => caze.setStatus("open")}>
          Reopen
        </Button>
      ) : (
        <Button size="small" leadingIcon={RiCheckLine} onClick={() => caze.setStatus("closed")}>
          Mark as done
        </Button>
      )}
    </div>
  );
}

function Refusal({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="text-body-2-regular text-text-error-primary">
      {error instanceof ApiError ? (Object.values(error.fields)[0] ?? error.message) : error.message}
    </p>
  );
}

export function DetailsPanel({
  workspace,
  conversation,
  caze,
  onClose,
}: {
  workspace: string;
  conversation: Conversation;
  caze: CaseControls;
  onClose: () => void;
}) {
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const people = members
    .filter((member) => member.status === "active")
    .map((member) => ({ id: member.id, name: member.user.name }));
  const customer = customerName(conversation.contact);

  return (
    <Panel label="Details" className="h-full w-full">
      <PageHeader title="Details" level={2}>
        <IconButton size="small" icon={RiCloseLine} aria-label="Close details" onClick={onClose} />
      </PageHeader>
      <ScrollArea className="px-4 pb-4">
        <div className="flex flex-col gap-2 pt-1 pb-2">
          <Actions conversation={conversation} caze={caze} me={caller?.member?.id ?? null} />
          <Refusal error={caze.error} />
          <CaseFields conversation={conversation} caze={caze} people={people} />
        </div>
        <Accordion
          variant="divided"
          size="sm"
          allowsMultipleExpanded
          defaultExpandedKeys={[section.customer]}
          className="border-t border-separator-border"
        >
          <AccordionItem id={section.customer} title="Customer" icon={RiUser3Line}>
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2.5">
                <Avatar size="md" color={avatarColor(customer)} initials={customer.charAt(0).toUpperCase()} />
                <div className="flex min-w-0 flex-col">
                  <span className="truncate text-body-2-medium text-text-primary">{customer}</span>
                  <span className="truncate text-body-2-regular text-text-tertiary">
                    {conversation.contact.email ?? "No email address"}
                  </span>
                </div>
              </div>
              <dl className="flex flex-col">
                <Row label="Kind">{conversation.contact.kind === "user" ? "User" : "Lead"}</Row>
                <Row label="Inbox">{conversation.inbox.name}</Row>
                <Row label="Channel">{conversation.channel.name}</Row>
              </dl>
            </div>
          </AccordionItem>
          <AccordionItem id={section.links} title="Links" icon={RiLinksLine}>
            <Nothing>Issues and tickets linked to this conversation show here.</Nothing>
          </AccordionItem>
          <AccordionItem id={section.articles} title="Help center articles" icon={RiArticleLine}>
            <Nothing>Articles dimmi used or suggested show here.</Nothing>
          </AccordionItem>
        </Accordion>
      </ScrollArea>
    </Panel>
  );
}
