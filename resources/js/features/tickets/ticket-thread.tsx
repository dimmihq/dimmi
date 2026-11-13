/*
 * A ticket with no conversation of its own, such as back-office work,
 * opened in the inbox the way a conversation opens: its thread in the
 * middle and its details beside it. The thread starts with what the
 * ticket is for, then runs oldest first through the notes the team left
 * each other and every change, with a box at the foot to add a note.
 * The header carries the reference, the title (click it to rename) and
 * the state. The details hold the state (an incident moves through its
 * stepper instead, which asks first), who holds it, priority, type,
 * fields, the customer requests it serves and the engineering work
 * behind it, each changed in place.
 */
import { RiArrowRightUpLine, RiChat3Line, RiStickyNoteLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type KeyboardEvent, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { ago } from "@/features/inbox/conversations";
import { Nothing, Row } from "@/features/inbox/customer/rows";
import { useAccount } from "@/features/settings/use-account";
import { avatarColor } from "@/features/shell/avatar-color";
import { Panel, ScrollArea } from "@/features/shell/page";
import { teamsQuery } from "@/lib/api/teams";
import {
  addTicketNote,
  type Ticket,
  type TicketEvent,
  type TicketPriority,
  ticketEventsQuery,
  ticketKeys,
  ticketPriorities,
} from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { TicketIssues } from "./issues/ticket-issues";
import { eventText, priorityLabel } from "./labels";
import { StatePill, TypeIcon } from "./state-pill";
import { ClashPrompt, FieldsEditor, StatePicker } from "./ticket-fields";
import { type TicketSave, useTicketSave } from "./use-ticket-save";
import { useTicketSetup } from "./use-ticket-setup";

const nobody = "none";

/*
 * TicketTitle reads as the heading and turns into a field when clicked;
 * Enter or leaving it renames the ticket, Escape keeps the old name.
 */
export function TicketTitle({ ticket, save }: { ticket: Ticket; save: TicketSave }) {
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(ticket.title);
  const done = (keep: boolean) => {
    setEditing(false);
    if (keep && title.trim() !== "" && title.trim() !== ticket.title) {
      void save.save({ title: title.trim() });
    } else {
      setTitle(ticket.title);
    }
  };

  if (editing) {
    return (
      <Input
        aria-label="Title"
        size="small"
        autoFocus
        value={title}
        maxLength={200}
        onChange={setTitle}
        onBlur={() => done(true)}
        onKeyDown={(event: KeyboardEvent) => {
          if (event.key === "Enter") {
            done(true);
          } else if (event.key === "Escape") {
            done(false);
          }
        }}
        className="min-w-0 flex-1"
      />
    );
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      title="Rename"
      className="min-w-0 truncate rounded-md px-1 text-start text-body-medium text-text-primary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      {ticket.title}
    </button>
  );
}

/*
 * ForLine says which customer request the ticket serves, linking to
 * its conversation.
 */
function ForLine({ ticket, view }: { ticket: Ticket; view: string }) {
  const parents = (ticket.links ?? []).filter((l) => l.kind === "back_office" && l.ticket && l.ticket.id !== ticket.id);
  if (parents.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-1.5 rounded-xl border border-border-button-default px-3 py-2.5">
      <span className="text-caption-1-medium text-text-tertiary">For</span>
      {parents.map((l) =>
        l.ticket ? <TicketLink key={l.id} view={view} ticket={l.ticket} conversation={l.conversation_id} /> : null,
      )}
    </div>
  );
}

function TicketLink({
  view,
  ticket,
  conversation,
}: {
  view: string;
  ticket: { id: string; reference: string; title: string; state: Ticket["state"] };
  conversation: string | null;
}) {
  return (
    <Link
      to="/d/inbox/$view/$conversation"
      params={{ view, conversation: conversation ?? ticket.id }}
      className="flex min-w-0 items-center gap-2 rounded-lg px-1 py-0.5 text-body-2-medium text-text-primary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <RiChat3Line aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="shrink-0 text-text-tertiary">{ticket.reference}</span>
      <span className="min-w-0 flex-1 truncate">{ticket.title}</span>
      <StatePill state={ticket.state} />
      <RiArrowRightUpLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
    </Link>
  );
}

/*
 * Note is one note a teammate left, shown like a message.
 */
function Note({ event }: { event: TicketEvent }) {
  const name = String(event.data.actor_name ?? "Teammate");

  return (
    <li className="flex gap-2.5">
      <Avatar size="sm" color={avatarColor(name)} initials={name.charAt(0)} aria-label={name} />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="flex items-baseline gap-2">
          <span className="text-body-2-medium text-text-primary">{name}</span>
          <span className="text-caption-1-regular text-text-tertiary">{ago(event.created_at)}</span>
        </span>
        <p className="w-fit max-w-full whitespace-pre-wrap rounded-xl rounded-ss-sm border border-status-yellow-text/40 bg-background-primary-default px-3 py-2 text-body-regular text-text-primary shadow-card">
          {String(event.data.body ?? "")}
        </p>
      </div>
    </li>
  );
}

/*
 * Change is one change to the ticket, a quiet line between the notes.
 */
function Change({ event }: { event: TicketEvent }) {
  const who = event.data.actor_name ? `${String(event.data.actor_name)} · ` : "";

  return (
    <li className="flex items-center gap-2 ps-1 text-caption-1-regular text-text-tertiary">
      <span aria-hidden className="size-1.5 shrink-0 rounded-full bg-foreground-icon-quaternary" />
      <span className="min-w-0 flex-1 truncate">
        {who}
        {eventText(event)}
      </span>
      <span className="shrink-0">{ago(event.created_at)}</span>
    </li>
  );
}

export function Timeline({
  workspace,
  ticket,
  view,
  intro = true,
}: {
  workspace: string;
  ticket: Ticket;
  view: string;
  intro?: boolean;
}) {
  const { data } = useQuery(ticketEventsQuery(workspace, ticket.id));
  const events = [...(data?.items ?? [])].sort((a, b) => a.created_at.localeCompare(b.created_at));

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-4 px-4 py-5">
      {intro ? <ForLine ticket={ticket} view={view} /> : null}
      {intro && ticket.description ? (
        <p className="whitespace-pre-wrap text-body-regular text-text-primary">{ticket.description}</p>
      ) : null}
      <ol aria-label="Notes and history" className="flex flex-col gap-3">
        {events.map((event) =>
          event.kind === "note" ? <Note key={event.id} event={event} /> : <Change key={event.id} event={event} />,
        )}
      </ol>
    </div>
  );
}

/*
 * NoteBox writes a note for the team at the foot of the thread; Cmd or
 * Ctrl+Enter adds it.
 */
export function NoteBox({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const client = useQueryClient();
  const [body, setBody] = useState("");
  const note = useMutation({
    mutationFn: () => addTicketNote(workspace, ticket.id, body.trim()),
    onSuccess: async () => {
      setBody("");
      await client.invalidateQueries({ queryKey: ticketKeys.events(workspace, ticket.id) });
    },
    onError: (error) => toast.error((error as Error).message),
  });

  return (
    <div className="mx-auto w-full max-w-[760px] px-4 pb-4">
      <div className="flex flex-col gap-2 rounded-2xl border border-border-button-default bg-background-primary-default p-2 shadow-xs">
        <Textarea
          aria-label="Note"
          placeholder="Leave a note for the team"
          rows={2}
          value={body}
          onChange={setBody}
          onKeyDown={(event: KeyboardEvent) => {
            if (event.key === "Enter" && (event.metaKey || event.ctrlKey) && body.trim() !== "") {
              event.preventDefault();
              note.mutate();
            }
          }}
          className="[&_textarea]:bg-transparent"
        />
        <div className="flex items-center justify-between gap-2 ps-1">
          <span className="flex items-center gap-1 text-caption-1-regular text-text-tertiary">
            <RiStickyNoteLine aria-hidden className="size-3.5" /> Only your team sees notes
          </span>
          <Button size="small" loading={note.isPending} disabled={body.trim() === ""} onClick={() => note.mutate()}>
            Add note
          </Button>
        </div>
      </div>
    </div>
  );
}

/*
 * Holders are who holds the ticket and how urgent it is.
 */
function Holders({ workspace, ticket, save }: { workspace: string; ticket: Ticket; save: TicketSave }) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const people = [
    { id: nobody, name: "Unassigned" },
    ...members.filter((m) => m.status === "active").map((m) => ({ id: m.id, name: m.user.name })),
  ];

  return (
    <>
      <Row label="Assignee">
        <Select
          aria-label="Assignee"
          size="sm"
          selectedKey={ticket.assignee_id ?? nobody}
          isDisabled={save.busy}
          onSelectionChange={(key) =>
            void save.save({ assignee_id: key === nobody || key === null ? null : String(key) })
          }
        >
          {people.map((p) => (
            <SelectItem key={p.id} id={p.id} textValue={p.name}>
              {p.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Team">
        <Select
          aria-label="Team"
          size="sm"
          selectedKey={ticket.team_id ?? nobody}
          isDisabled={save.busy}
          onSelectionChange={(key) => void save.save({ team_id: key === nobody || key === null ? null : String(key) })}
        >
          {[{ id: nobody, name: "No team" }, ...teams].map((t) => (
            <SelectItem key={t.id} id={t.id} textValue={t.name}>
              {t.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Priority">
        <Select
          aria-label="Priority"
          size="sm"
          selectedKey={ticket.priority}
          isDisabled={save.busy}
          onSelectionChange={(key) => key !== null && void save.save({ priority: String(key) as TicketPriority })}
        >
          {ticketPriorities.map((p) => (
            <SelectItem key={p} id={p} textValue={priorityLabel(p)}>
              {priorityLabel(p)}
            </SelectItem>
          ))}
        </Select>
      </Row>
    </>
  );
}

function Block({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section aria-label={label} className="flex flex-col gap-2 border-t border-separator-border pt-3">
      <h3 className="text-caption-1-medium text-text-tertiary">{label}</h3>
      {children}
    </section>
  );
}

/*
 * TicketDetails is the column beside the thread.
 */
export function TicketDetails({
  workspace,
  ticket,
  save,
  children,
}: {
  workspace: string;
  ticket: Ticket;
  save: TicketSave;
  children?: React.ReactNode;
}) {
  const setup = useTicketSetup(workspace, ticket);
  const hasFields = (setup.type?.fields ?? []).some((f) => !f.archived_at);

  return (
    <div className="flex flex-col gap-3 px-4 pt-1 pb-4">
      <ClashPrompt save={save} />
      {ticket.kind === "incident" ? null : (
        <div className="flex flex-col gap-1">
          <StatePicker ticket={ticket} states={setup.states} save={save} compact />
        </div>
      )}
      <dl className="flex flex-col">
        <Holders workspace={workspace} ticket={ticket} save={save} />
        <Row label="Type">
          <span className="flex min-w-0 items-center gap-1.5">
            <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
            <span className="truncate">{ticket.type.name}</span>
          </span>
        </Row>
        <Row label="Created">{ago(ticket.created_at)}</Row>
      </dl>
      {children}
      <Block label="Fields">
        {hasFields ? (
          <FieldsEditor key={ticket.version} workspace={workspace} ticket={ticket} type={setup.type} save={save} />
        ) : (
          <Nothing>This type has no fields.</Nothing>
        )}
      </Block>
      <Block label="Engineering work">
        <TicketIssues ticketId={ticket.id} compact />
      </Block>
    </div>
  );
}

export function TicketThread({ view, ticket }: { view: string; ticket: Ticket }) {
  const workspace = useAccount().workspace.id;
  const save = useTicketSave(workspace, ticket);

  return (
    <div className="flex h-full min-w-0 flex-1 gap-2">
      <Panel label={`${ticket.reference} ${ticket.title}`} className="flex h-full min-w-0 flex-1 flex-col">
        <header className="flex h-12 shrink-0 items-center gap-2 ps-3 pe-2.5 pt-[7px]">
          <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
          <span className="shrink-0 text-body-2-medium text-text-tertiary">{ticket.reference}</span>
          <TicketTitle key={ticket.version} ticket={ticket} save={save} />
          <span className="ms-auto shrink-0">
            <StatePill state={ticket.state} />
          </span>
        </header>
        <ScrollArea className="flex-1">
          <Timeline workspace={workspace} ticket={ticket} view={view} />
        </ScrollArea>
        <NoteBox workspace={workspace} ticket={ticket} />
      </Panel>
      <Panel label="Details" className={cx("hidden h-full w-[340px] shrink-0 flex-col lg:flex")}>
        <header className="flex h-12 shrink-0 items-center ps-4 pe-2.5 pt-[7px]">
          <h2 className="text-body-medium text-text-primary">Details</h2>
        </header>
        <ScrollArea>
          <TicketDetails workspace={workspace} ticket={ticket} save={save} />
        </ScrollArea>
      </Panel>
    </div>
  );
}
