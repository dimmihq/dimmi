/*
 * A ticket with no conversation of its own, such as back-office work,
 * opened in the inbox the way a conversation opens: its thread in the
 * middle and its details beside it. The thread starts with what the
 * ticket is for, then runs oldest first through the notes the team left
 * each other and every change, with a box at the foot to add a note.
 * The header carries the reference, the title (click it to rename) and
 * the state. The details hold the state, who holds it, priority, type,
 * fields, the customer requests it serves and the engineering work
 * behind it, each changed in place.
 */
import { RiArrowRightUpLine, RiChat3Line, RiLock2Line, RiStickyNoteLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type KeyboardEvent, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Accordion } from "@/components/base/accordion/accordion";
import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { AssignPicker } from "@/features/inbox/assign/assign-picker";
import { NoteComposer } from "@/features/inbox/composer";
import { ago, toolOf } from "@/features/inbox/conversations";
import { quiet } from "@/features/inbox/customer/quiet";
import { Row, Section } from "@/features/inbox/customer/rows";
import { MessageBody } from "@/features/inbox/message-body";
import { useAccount } from "@/features/settings/use-account";
import { avatarColor } from "@/features/shell/avatar-color";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useWide } from "@/features/shell/use-wide";
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

import { foldLinks } from "./fold-links";
import { TicketIssues } from "./issues/ticket-issues";
import { ToolMark } from "./issues/tool-logos";
import { eventText, priorityLabel } from "./labels";
import { StatePill, TypeIcon } from "./state-pill";
import { ClashPrompt, FieldsEditor, StateMenu } from "./ticket-fields";
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
 * Note is one note a teammate left, drawn as the conversation thread
 * draws an internal note: on the team's side, in the same rounded
 * bubble with the note's edge, its text read as Markdown, and who wrote
 * it and when underneath.
 */
export function Note({ event }: { event: TicketEvent }) {
  const name = String(event.data.actor_name ?? "Teammate");
  const tool = toolOf(event.actor_type, name);

  return (
    <li aria-label={`Internal note from ${name}`} className="flex flex-col items-end gap-1.5">
      <div className="w-fit max-w-[min(520px,calc(100%-24px))] rounded-[var(--radius-2-5xl)] border border-status-yellow-text/40 bg-background-primary-default p-3 text-body-regular text-text-primary shadow-card">
        <MessageBody text={String(event.data.body ?? "")} />
      </div>
      <span className="flex items-center gap-1.5 text-body-2-regular text-text-tertiary">
        {tool ? (
          <ToolMark provider={tool} className="size-4" />
        ) : (
          <RiLock2Line aria-hidden className="size-4 text-foreground-icon-tertiary" />
        )}
        <span className="text-text-secondary">{name}</span>
        {ago(event.created_at)}
      </span>
    </li>
  );
}

/*
 * Change is one change to the ticket, a quiet line between the notes.
 */
export function Change({ event }: { event: TicketEvent }) {
  return (
    <li className="flex items-center gap-2 ps-1 text-caption-1-regular text-text-tertiary">
      <span aria-hidden className="size-1.5 shrink-0 rounded-full bg-foreground-icon-quaternary" />
      <span className="min-w-0 flex-1 truncate">{eventText(event)}</span>
      <span className="shrink-0">{ago(event.created_at)}</span>
    </li>
  );
}

/*
 * LinkRun is a run of links made one after another, such as dozens of
 * conversations joining a tracker, folded into one quiet line.
 */
export function LinkRun({ events }: { events: TicketEvent[] }) {
  const first = events[0];
  const last = events[events.length - 1];
  if (!first || !last) {
    return null;
  }

  return (
    <li className="flex items-center gap-2 ps-1 text-caption-1-regular text-text-tertiary">
      <span aria-hidden className="size-1.5 shrink-0 rounded-full bg-foreground-icon-quaternary" />
      <span className="min-w-0 flex-1 truncate">
        {events.length === 1 ? eventText(first) : `${events.length} conversations and tickets linked`}
      </span>
      <span className="shrink-0">{ago(last.created_at)}</span>
    </li>
  );
}

export function Timeline({ workspace, ticket, view }: { workspace: string; ticket: Ticket; view: string }) {
  const { data } = useQuery(ticketEventsQuery(workspace, ticket.id));
  const events = [...(data?.items ?? [])].sort((a, b) => a.created_at.localeCompare(b.created_at));

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-4 px-4 py-5">
      <ForLine ticket={ticket} view={view} />
      {ticket.description ? (
        <p className="whitespace-pre-wrap text-body-regular text-text-primary">{ticket.description}</p>
      ) : null}
      <ol aria-label="Notes and history" className="flex flex-col gap-3">
        {foldLinks(events, (event) => (event.kind === "link" ? event : null)).map((item) => {
          if (Array.isArray(item)) {
            return <LinkRun key={item[0]?.id} events={item} />;
          }

          return item.kind === "note" ? <Note key={item.id} event={item} /> : <Change key={item.id} event={item} />;
        })}
      </ol>
    </div>
  );
}

/*
 * NoteBox writes a note for the team. At the foot of a ticket's thread
 * it is the conversation's composer with only its note side; bare, in a
 * card inside a details panel, it is a plain field. Cmd or Ctrl+Enter
 * adds the note.
 */
export function NoteBox({ workspace, ticket, bare = false }: { workspace: string; ticket: Ticket; bare?: boolean }) {
  const client = useQueryClient();
  const [body, setBody] = useState("");
  const refresh = () => client.invalidateQueries({ queryKey: ticketKeys.events(workspace, ticket.id) });
  const note = useMutation({
    mutationFn: () => addTicketNote(workspace, ticket.id, body.trim()),
    onSuccess: async () => {
      setBody("");
      await refresh();
    },
    onError: (error) => toast.error((error as Error).message),
  });

  if (!bare) {
    return (
      <div className="px-3 pb-3">
        <NoteComposer onAdd={(text) => addTicketNote(workspace, ticket.id, text).then(refresh)} />
      </div>
    );
  }

  return (
    <div className="w-full">
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
export function Holders({ workspace, ticket, save }: { workspace: string; ticket: Ticket; save: TicketSave }) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const [assigning, setAssigning] = useState(false);
  const holder = members.find((m) => m.id === ticket.assignee_id)?.user.name ?? null;

  return (
    <>
      <Row label="Assignee">
        <AssignPicker
          workspace={workspace}
          assignee={ticket.assignee_id}
          team={null}
          isOpen={assigning}
          onOpenChange={setAssigning}
          onMember={(id) => void save.save({ assignee_id: id })}
          trigger={{
            label: `Assignee: ${holder ?? "Unassigned"}`,
            className: quiet,
            children: holder ? (
              <span className="flex min-w-0 items-center gap-2">
                <Avatar size="xs" color={avatarColor(holder)} initials={holder.charAt(0)} />
                <span className="truncate">{holder}</span>
              </span>
            ) : (
              "Unassigned"
            ),
          }}
        />
      </Row>
      <Row label="Team">
        <Select
          aria-label="Team"
          size="sm"
          variant="quiet"
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
          variant="quiet"
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

/*
 * TicketDetails is the column beside the thread, built like the
 * conversation's details: the state, who holds it and what it is as
 * rows, then sections. What a kind adds, such as a tracker's customer
 * reports, comes in as the first section; `open` names the sections it
 * adds so they start open.
 */
export function TicketDetails({
  workspace,
  ticket,
  save,
  open = [],
  children,
}: {
  workspace: string;
  ticket: Ticket;
  save: TicketSave;
  open?: string[];
  children?: React.ReactNode;
}) {
  const setup = useTicketSetup(workspace, ticket);
  const hasFields = (setup.type?.fields ?? []).some((f) => !f.archived_at);
  const sees = setup.states.find((s) => s.id === ticket.state_id)?.customer_label ?? ticket.state.customer_label;

  return (
    <Accordion
      variant="plain"
      size="sm"
      allowsMultipleExpanded
      defaultExpandedKeys={[...open, "Fields", "Issues"]}
      className="pt-1"
    >
      <div className="flex flex-col gap-1">
        <ClashPrompt save={save} />
        <dl className="flex flex-col">
          <Row label="State">
            <StateMenu ticket={ticket} states={setup.states} save={save} />
          </Row>
          {sees ? <Row label="Customer sees">{sees}</Row> : null}
          <Holders workspace={workspace} ticket={ticket} save={save} />
          <Row label="Type">
            <span className="flex min-w-0 items-center gap-1.5">
              <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
              <span className="truncate">{ticket.type.name}</span>
            </span>
          </Row>
          <Row label="Created">{ago(ticket.created_at)}</Row>
        </dl>
      </div>
      {children}
      {hasFields ? (
        <Section title="Fields">
          <FieldsEditor key={ticket.version} workspace={workspace} ticket={ticket} type={setup.type} save={save} />
        </Section>
      ) : null}
      <TicketIssues ticketId={ticket.id} compact heading="Issues" />
    </Accordion>
  );
}

/*
 * DetailsColumn is the panel that holds the details.
 */
export function DetailsColumn({ children }: { children: React.ReactNode }) {
  return (
    <Panel label="Details" className="h-full w-full">
      <PageHeader title="Details" level={2} />
      <ScrollArea className="px-4 pb-6">{children}</ScrollArea>
    </Panel>
  );
}

const visible = { overflow: "visible" } as const;
const split = { group: "dimmi:ticket", thread: "thread", details: "details" } as const;

/*
 * TicketSplit lays a ticket's thread and its details side by side, with
 * the gap between them a grip that resizes them the way a conversation's
 * panels resize; each browser remembers the width. On a narrow screen
 * only the thread shows.
 */
export function TicketSplit({ thread, details }: { thread: React.ReactNode; details: React.ReactNode }) {
  const sideBySide = useWide(1024);
  if (!sideBySide) {
    return <div className="flex h-full min-w-0 flex-1">{thread}</div>;
  }

  return (
    <PaneGroup id={split.group} panelIds={[split.thread, split.details]}>
      <Pane id={split.thread} minSize={360} style={visible} className="flex h-full">
        {thread}
      </Pane>
      <PaneHandle />
      <Pane
        id={split.details}
        defaultSize={360}
        minSize={300}
        maxSize={560}
        groupResizeBehavior="preserve-pixel-size"
        style={visible}
        className="flex h-full"
      >
        {details}
      </Pane>
    </PaneGroup>
  );
}

export function TicketThread({ view, ticket }: { view: string; ticket: Ticket }) {
  const workspace = useAccount().workspace.id;
  const save = useTicketSave(workspace, ticket);

  return (
    <TicketSplit
      thread={
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
      }
      details={
        <DetailsColumn>
          <TicketDetails workspace={workspace} ticket={ticket} save={save} />
        </DetailsColumn>
      }
    />
  );
}
