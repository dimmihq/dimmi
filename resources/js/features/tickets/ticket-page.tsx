/*
 * The page of a ticket that has no conversation of its own, such as
 * back-office work: its title, state, who holds it, priority and fields,
 * all editable, then notes the team leaves for each other and the full
 * history of what changed, newest first. A customer ticket shown here
 * links back to its conversation, where its replies live. Edits carry
 * the version read, so a clash offers to load the latest.
 */
import { RiArrowLeftSLine, RiChat3Line, RiCoupon3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useRouter } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { ago } from "@/features/inbox/conversations";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel, ScrollArea } from "@/features/shell/page";
import { teamsQuery } from "@/lib/api/teams";
import {
  addTicketNote,
  type Ticket,
  type TicketPriority,
  ticketEventsQuery,
  ticketKeys,
  ticketPriorities,
} from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";
import { TicketIssues } from "./issues/ticket-issues";
import { eventText, priorityLabel } from "./labels";
import { TypeIcon } from "./state-pill";
import { ClashPrompt, FieldsEditor, StatePicker } from "./ticket-fields";
import { type TicketSave, useTicketSave } from "./use-ticket-save";
import { useTicketSetup } from "./use-ticket-setup";

const nobody = "none";

function Title({ ticket, save }: { ticket: Ticket; save: TicketSave }) {
  const [title, setTitle] = useState(ticket.title);

  return (
    <Input
      aria-label="Title"
      value={title}
      onChange={setTitle}
      maxLength={200}
      onBlur={() => {
        if (title.trim() !== "" && title !== ticket.title) {
          void save.save({ title: title.trim() });
        }
      }}
    />
  );
}

function Holders({ workspace, ticket, save }: { workspace: string; ticket: Ticket; save: TicketSave }) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const people = [
    { id: nobody, name: "Unassigned" },
    ...members.filter((m) => m.status === "active").map((m) => ({ id: m.id, name: m.user.name })),
  ];

  return (
    <div className="grid gap-3 sm:grid-cols-3">
      <Select
        label="Assignee"
        size="sm"
        selectedKey={ticket.assignee_id ?? nobody}
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
      <Select
        label="Team"
        size="sm"
        selectedKey={ticket.team_id ?? nobody}
        onSelectionChange={(key) => void save.save({ team_id: key === nobody || key === null ? null : String(key) })}
      >
        {[{ id: nobody, name: "No team" }, ...teams].map((t) => (
          <SelectItem key={t.id} id={t.id} textValue={t.name}>
            {t.name}
          </SelectItem>
        ))}
      </Select>
      <Select
        label="Priority"
        size="sm"
        selectedKey={ticket.priority}
        onSelectionChange={(key) => key !== null && void save.save({ priority: String(key) as TicketPriority })}
      >
        {ticketPriorities.map((p) => (
          <SelectItem key={p} id={p} textValue={priorityLabel(p)}>
            {priorityLabel(p)}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}

function Activity({ workspace, ticket }: { workspace: string; ticket: Ticket }) {
  const client = useQueryClient();
  const { data } = useQuery(ticketEventsQuery(workspace, ticket.id));
  const [body, setBody] = useState("");
  const note = useMutation({
    mutationFn: () => addTicketNote(workspace, ticket.id, body.trim()),
    onSuccess: async () => {
      setBody("");
      await client.invalidateQueries({ queryKey: ticketKeys.events(workspace, ticket.id) });
    },
    onError: (error) => toast.error((error as Error).message),
  });
  const events = [...(data?.items ?? [])].reverse();

  return (
    <section aria-label="Notes and history" className="flex flex-col gap-3">
      <h2 className="text-headline-medium text-text-primary">Notes and history</h2>
      <Textarea aria-label="Note" placeholder="Leave a note for the team" rows={3} value={body} onChange={setBody} />
      <div className="flex justify-end">
        <Button size="small" loading={note.isPending} disabled={body.trim() === ""} onClick={() => note.mutate()}>
          Add note
        </Button>
      </div>
      <ol className="flex flex-col">
        {events.map((e) => (
          <li key={e.id} className="flex gap-3 border-b border-separator-border py-2.5 last:border-b-0">
            <span className="w-16 shrink-0 text-caption-1-medium text-text-tertiary">{ago(e.created_at)}</span>
            <span
              className={
                e.kind === "note"
                  ? "min-w-0 flex-1 whitespace-pre-wrap text-body-2-regular text-text-primary"
                  : "min-w-0 flex-1 text-body-2-regular text-text-secondary"
              }
            >
              {e.kind === "note" && e.data.actor_name ? (
                <span className="text-body-2-medium text-text-primary">{String(e.data.actor_name)}: </span>
              ) : null}
              {eventText(e)}
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}

export function TicketPage({ ticket }: { ticket: Ticket }) {
  const workspace = useAccount().workspace.id;
  const router = useRouter();
  const save = useTicketSave(workspace, ticket);
  const setup = useTicketSetup(workspace, ticket);

  return (
    <Panel label={`${ticket.reference} ${ticket.title}`} ground className="h-full flex-1">
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-[720px] flex-col gap-6 px-4 pt-6 pb-16 sm:px-6 sm:pt-10">
          <header className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <button
                type="button"
                aria-label="Back"
                onClick={() => router.history.back()}
                className="flex size-8 items-center justify-center rounded-lg text-foreground-icon-secondary hover:bg-background-secondary-hover"
              >
                <RiArrowLeftSLine aria-hidden className="size-5" />
              </button>
              <TypeIcon icon={ticket.type.icon} kind={ticket.kind} />
              <span className="text-body-2-medium text-text-secondary">
                {ticket.reference} · {ticket.type.name}
              </span>
              {ticket.conversation_id ? (
                <Link
                  to="/d/inbox/$view/$conversation"
                  params={{ view: "all", conversation: ticket.conversation_id }}
                  className="ms-auto flex items-center gap-1 text-body-2-medium text-text-primary underline"
                >
                  <RiChat3Line aria-hidden className="size-4" /> Open conversation
                </Link>
              ) : null}
            </div>
            <Title key={ticket.version} ticket={ticket} save={save} />
          </header>
          <ClashPrompt save={save} />
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <StatePicker ticket={ticket} states={setup.states} save={save} />
          </div>
          <Holders workspace={workspace} ticket={ticket} save={save} />
          {ticket.description ? (
            <p className="whitespace-pre-wrap text-body-regular text-text-primary">{ticket.description}</p>
          ) : null}
          <section aria-label="Fields" className="flex flex-col gap-3">
            <h2 className="text-headline-medium text-text-primary">Fields</h2>
            <FieldsEditor key={ticket.version} workspace={workspace} ticket={ticket} type={setup.type} save={save} />
          </section>
          <TicketIssues ticketId={ticket.id} />
          <Activity workspace={workspace} ticket={ticket} />
        </div>
      </ScrollArea>
    </Panel>
  );
}

export function TicketMissing() {
  return (
    <Panel label="Ticket" className="flex-1">
      <EmptyState
        icon={RiCoupon3Line}
        title="This ticket is not here"
        description="It may have been deleted, or it is not yours to see."
      />
    </Panel>
  );
}
