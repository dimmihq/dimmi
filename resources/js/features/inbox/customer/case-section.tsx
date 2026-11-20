/*
 * The moves a teammate makes on the case: assign it, change its status or
 * priority, mark it done or snooze it, and tag it. Each value reads as
 * text and opens its choices when pressed. Tags are the one
 * vocabulary the workspace shares; Shift+T opens the picker from
 * anywhere in the conversation.
 */
import { RiArrowDownSLine, RiArrowGoBackLine, RiCheckLine, RiTimeLine, RiUserSharedLine } from "@remixicon/react";
import { type Key, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { linkButtonStyles } from "@/components/base/buttons/link-button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Select, SelectItem } from "@/components/base/select/select";
import { TeamRow } from "@/features/inbox/team-select";
import { avatarColor } from "@/features/shell/avatar-color";
import { ApiError } from "@/lib/api/client";
import {
  type Conversation,
  type ConversationPriority,
  type ConversationStatus,
  conversationPriorities,
} from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { AssignPicker } from "../assign/assign-picker";
import { type CaseControls, statusLabel } from "../case";
import { priorityLabel } from "../priority/labels";
import { TagChip } from "../tags/tag-chip";
import { TagPicker } from "../tags/tag-picker";
import { quiet } from "./quiet";
import { Row, Section } from "./rows";

const statusDot = { open: "green", snoozed: "yellow", closed: "indigo" } as const;

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
  workspace,
  conversation,
  caze,
}: {
  workspace: string;
  conversation: Conversation;
  caze: CaseControls;
}) {
  const [assigning, setAssigning] = useState(false);
  const assignee = conversation.assignee;

  return (
    <dl className="flex flex-col">
      <Row label="Assignee">
        <AssignPicker
          workspace={workspace}
          assignee={assignee?.id ?? null}
          team={null}
          isOpen={assigning}
          onOpenChange={setAssigning}
          onMember={caze.assign}
          trigger={{
            label: `Assignee: ${assignee?.name ?? "Unassigned"}`,
            className: quiet,
            children: assignee ? <PersonName name={assignee.name} /> : "Unassigned",
          }}
        />
      </Row>
      <TeamRow conversation={conversation} caze={caze} />
      <Row label="Status">
        <Select
          aria-label="Status"
          size="sm"
          variant="quiet"
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
          variant="quiet"
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
      <Row label="Channel">{channelName(conversation.channel)}</Row>
    </dl>
  );
}

/*
 * Actions gathers the case's moves in one menu beside the panel's close
 * button: take the conversation, snooze it, and mark it done or reopen
 * it. Each move closes the menu.
 */
export function Actions({
  conversation,
  caze,
  me,
}: {
  conversation: Conversation;
  caze: CaseControls;
  me: string | null;
}) {
  const [open, setOpen] = useState(false);
  const run = (move: () => void) => {
    setOpen(false);
    move();
  };
  const closed = conversation.status === "closed";

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        className={cx(linkButtonStyles.base, linkButtonStyles.size.small, linkButtonStyles.variant.secondary)}
      >
        Actions
        <RiArrowDownSLine
          aria-hidden
          className={cx(linkButtonStyles.icon.small, "transition-transform duration-150", open && "rotate-180")}
        />
      </DropdownTrigger>
      <DropdownPopover aria-label="Conversation actions" placement="bottom end" className="w-52 p-1.5">
        {me === null || conversation.assignee?.id === me ? null : (
          <DropdownItem onSelect={() => run(() => caze.assign(me))} className="gap-2 px-2 py-1.5">
            <RiUserSharedLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            <span className="text-body-2-medium">Assign to me</span>
          </DropdownItem>
        )}
        {conversation.status === "open" ? (
          <DropdownItem onSelect={() => run(() => caze.setStatus("snoozed"))} className="gap-2 px-2 py-1.5">
            <RiTimeLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            <span className="text-body-2-medium">Snooze</span>
          </DropdownItem>
        ) : null}
        <DropdownItem
          onSelect={() => run(() => caze.setStatus(closed ? "open" : "closed"))}
          className="gap-2 px-2 py-1.5"
        >
          {closed ? (
            <RiArrowGoBackLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          ) : (
            <RiCheckLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          )}
          <span className="text-body-2-medium">{closed ? "Reopen" : "Mark as done"}</span>
        </DropdownItem>
      </DropdownPopover>
    </Dropdown>
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
    <Section title="Tags">
      <div className="flex flex-wrap items-center gap-1">
        {conversation.tags.map((tag) => (
          <TagChip key={tag.id} tag={tag} size="small" onRemove={() => caze.removeTag(tag.id)} />
        ))}
        <TagPicker
          workspace={workspace}
          label="Add tag"
          selected={conversation.tags}
          onAdd={caze.addTag}
          onRemove={(tag) => caze.removeTag(tag.id)}
          shortcut
          quiet
        />
      </div>
    </Section>
  );
}

/*
 * platformNames are the channels whose connections are named after an
 * account, which alone would not say where the conversation came from.
 */
const platformNames: Record<string, string> = {
  whatsapp: "WhatsApp",
  facebook: "Messenger",
  instagram: "Instagram",
};

function channelName(channel: { type: string; name: string }): string {
  const platform = platformNames[channel.type];

  return platform ? `${platform} · ${channel.name}` : channel.name;
}
