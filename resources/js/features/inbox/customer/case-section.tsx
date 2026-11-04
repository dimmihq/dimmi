/*
 * The moves a teammate makes on the case: assign it, change its status or
 * priority, mark it done or snooze it, and tag it. Tags are the one
 * vocabulary the workspace shares; Shift+T opens the picker from
 * anywhere in the conversation.
 */
import { RiCheckLine, RiTimeLine, RiUserSharedLine } from "@remixicon/react";
import type { Key } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { avatarColor } from "@/features/shell/avatar-color";
import { ApiError } from "@/lib/api/client";
import {
  type Conversation,
  type ConversationPriority,
  type ConversationStatus,
  conversationPriorities,
} from "@/lib/api/inbox";

import { type CaseControls, statusLabel } from "../case";
import { priorityLabel } from "../priority/labels";
import { TagChip } from "../tags/tag-chip";
import { TagPicker } from "../tags/tag-picker";
import { Row } from "./rows";

const nobody = "nobody";
const statusDot = { open: "green", snoozed: "yellow", closed: "indigo" } as const;

export interface Person {
  id: string;
  name: string;
}

function PersonName({ name }: { name: string }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <Avatar size="xs" color={avatarColor(name)} initials={name.charAt(0)} />
      <span className="truncate">{name}</span>
    </span>
  );
}

function choose<T extends string>(apply: (value: T) => void) {
  return (key: Key | null) => {
    if (key !== null) {
      apply(String(key) as T);
    }
  };
}

export function CaseFields({
  conversation,
  caze,
  people,
}: {
  conversation: Conversation;
  caze: CaseControls;
  people: Person[];
}) {
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
          selectedKey={conversation.priority}
          onSelectionChange={choose<ConversationPriority>(caze.setPriority)}
        >
          {conversationPriorities.map((value) => (
            <SelectItem key={value} id={value} textValue={priorityLabel[value]}>
              {priorityLabel[value]}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Inbox">{conversation.inbox.name}</Row>
      <Row label="Channel">{conversation.channel.name}</Row>
    </dl>
  );
}

export function Actions({
  conversation,
  caze,
  me,
}: {
  conversation: Conversation;
  caze: CaseControls;
  me: string | null;
}) {
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

export function Refusal({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="text-body-2-regular text-text-error-primary">
      {error instanceof ApiError ? (Object.values(error.fields)[0] ?? error.message) : error.message}
    </p>
  );
}

export function CaseTags({
  workspace,
  conversation,
  caze,
}: {
  workspace: string;
  conversation: Conversation;
  caze: CaseControls;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {conversation.tags.map((tag) => (
        <TagChip key={tag.id} tag={tag} onRemove={() => caze.removeTag(tag.id)} />
      ))}
      <TagPicker
        workspace={workspace}
        label="Add tag"
        selected={conversation.tags}
        onAdd={caze.addTag}
        onRemove={(tag) => caze.removeTag(tag.id)}
        shortcut
      />
    </div>
  );
}
