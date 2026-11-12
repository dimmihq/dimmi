/*
 * The list of an inbox view: one row per conversation with the channel,
 * the customer, who is viewing it, the subject, how long the customer
 * has waited, whether the reader left a draft there, its priority and
 * who holds it, the agent's part in it, and the inbox when the reader
 * sees several, in the view's order. Rows
 * follow BoardUI's chat list: the open conversation sits on the tertiary
 * surface, the rest light up on hover. A conversation with something the
 * reader has not seen carries a dot and reads in the stronger weight.
 * Hovering a row turns its avatar into a checkbox that picks it for a
 * bulk change; shift-click picks a range and X picks the focused row.
 * While anything is picked, every row shows its checkbox.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type KeyboardEvent, useRef } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { avatarColor } from "@/features/shell/avatar-color";
import { type Conversation, inboxesQuery } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { AiBadge, ApprovalBadge } from "./ai-badge";
import type { SelectionControls } from "./bulk/selection";
import { ago, customerName, subjectOf, useInbox } from "./conversations";
import { PriorityChip } from "./priority/priority-menu";
import { AssigneeAvatar, ChannelBadge, DraftMark, InboxLabel, Viewers, Waiting } from "./row-extras";

export function ConversationList({
  view,
  conversations,
  openId,
  me,
  drafted,
  selection,
}: {
  view: string;
  conversations: Conversation[];
  openId?: string;
  me?: string;
  drafted?: ReadonlySet<string>;
  selection?: SelectionControls;
}) {
  const { workspace } = useInbox();
  const { data: inboxes = [] } = useQuery({ ...inboxesQuery(workspace), enabled: workspace !== "" });
  const selecting = (selection?.count ?? 0) > 0;

  return (
    <ul
      className="flex flex-col gap-1 px-[7px] pb-[7px]"
      onKeyDown={(event) => {
        if (event.key === "Escape" && selecting) {
          selection?.clear();
        }
      }}
    >
      {conversations.map((conversation) => (
        <Row
          key={conversation.id}
          view={view}
          conversation={conversation}
          open={conversation.id === openId}
          me={me}
          drafted={drafted?.has(conversation.id) ?? false}
          inboxes={inboxes}
          selection={selection}
        />
      ))}
    </ul>
  );
}

/*
 * pickOnX picks or drops the focused row on X, the range with Shift.
 */
function pickOnX(selection: SelectionControls | undefined, id: string) {
  return (event: KeyboardEvent<HTMLAnchorElement>) => {
    if (selection && event.key.toLowerCase() === "x" && !event.metaKey && !event.ctrlKey && !event.altKey) {
      event.preventDefault();
      selection.toggle(id, event.shiftKey);
    }
  };
}

function rowTone(open: boolean, picked: boolean): string {
  if (open) {
    return "bg-background-tertiary-default";
  }

  return picked ? "bg-background-secondary-default" : "hover:bg-background-secondary-hover";
}

function Row({
  view,
  conversation,
  open,
  me,
  drafted,
  inboxes,
  selection,
}: {
  view: string;
  conversation: Conversation;
  open: boolean;
  me?: string;
  drafted: boolean;
  inboxes: Parameters<typeof InboxLabel>[0]["inboxes"];
  selection?: SelectionControls;
}) {
  const name = customerName(conversation.contact);
  const picked = selection?.ids.has(conversation.id) ?? false;
  const selecting = (selection?.count ?? 0) > 0;

  return (
    <li className="group/row relative">
      <Link
        to="/d/inbox/$view/$conversation"
        params={{ view, conversation: conversation.id }}
        aria-current={open ? "page" : undefined}
        onKeyDown={pickOnX(selection, conversation.id)}
        className={cx(
          "flex items-center gap-3 rounded-xl px-2.5 py-2.5 outline-none transition-colors duration-150",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          rowTone(open, picked),
        )}
      >
        <span className={cx("relative shrink-0", selection && (selecting ? "invisible" : "group-hover/row:invisible"))}>
          <Avatar size="lg" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />
          <ChannelBadge channel={conversation.channel} />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="flex items-center gap-2">
            <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{name}</span>
            <Viewers viewers={conversation.viewers} me={me} />
            <span className="shrink-0 text-caption-1-medium text-text-tertiary">
              {ago(conversation.last_message_at ?? conversation.created_at)}
            </span>
          </span>
          <span className="flex items-center gap-2">
            <span
              className={cx(
                "min-w-0 flex-1 truncate text-body-2-regular",
                conversation.unread ? "text-text-primary" : "text-text-tertiary",
              )}
            >
              {subjectOf(conversation)}
            </span>
            <DraftMark drafted={drafted} />
            <Waiting conversation={conversation} />
            <AiBadge state={conversation.ai_state} />
            <ApprovalBadge count={conversation.ai_pending_approvals} />
            <PriorityChip priority={conversation.priority} />
            <AssigneeAvatar assignee={conversation.assignee} />
            {conversation.unread ? (
              <>
                <StatusDot color="indigo" />
                <span className="sr-only">Unread</span>
              </>
            ) : null}
          </span>
          <InboxLabel inbox={conversation.inbox} inboxes={inboxes} />
        </span>
      </Link>
      {selection ? (
        <PickBox
          name={name}
          picked={picked}
          shown={selecting}
          onToggle={(range) => selection.toggle(conversation.id, range)}
        />
      ) : null}
    </li>
  );
}

/*
 * PickBox is the checkbox over a row's avatar, shown on hover or while
 * anything is picked. A shift-click passes range.
 */
function PickBox({
  name,
  picked,
  shown,
  onToggle,
}: {
  name: string;
  picked: boolean;
  shown: boolean;
  onToggle: (range: boolean) => void;
}) {
  const shift = useRef(false);

  return (
    <span
      className={cx(
        "absolute start-2.5 top-2.5 size-9 items-center justify-center",
        shown ? "flex" : "hidden group-hover/row:flex",
      )}
      onPointerDownCapture={(event) => {
        shift.current = event.shiftKey;
      }}
    >
      <Checkbox
        aria-label={`Select ${name}`}
        isSelected={picked}
        onChange={() => {
          onToggle(shift.current);
          shift.current = false;
        }}
      />
    </span>
  );
}
