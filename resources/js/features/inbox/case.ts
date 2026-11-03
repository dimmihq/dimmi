/*
 * The state of a conversation that teammates change while working it:
 * who it is assigned to, its status and its priority. Each change also
 * lands in the thread as an event, so the history says who did what.
 * Until the inbox is connected, changes live in this page only.
 */
import { useState } from "react";

import type { Conversation, ThreadEntry } from "./conversations";

export type Status = Conversation["status"];
export type Priority = Conversation["priority"];

export const statusLabel: Record<Status, string> = { open: "Open", snoozed: "Snoozed", done: "Done" };
export const priorityLabel: Record<Priority, string> = { low: "Low", normal: "Normal", high: "High", urgent: "Urgent" };

export interface Case {
  assignee: string | null;
  status: Status;
  priority: Priority;
  events: ThreadEntry[];
}

/*
 * useCase holds a conversation's case and the moves a teammate (actor)
 * makes on it.
 */
export function useCase(conversation: Conversation, actor: string) {
  const [state, setState] = useState<Case>({
    assignee: conversation.assignee,
    status: conversation.status,
    priority: conversation.priority,
    events: [],
  });

  const record = (change: Partial<Omit<Case, "events">>, text: string) =>
    setState((before) => ({
      ...before,
      ...change,
      events: [...before.events, { kind: "event", id: `case-${before.events.length}`, at: "Just now", text }],
    }));

  return {
    ...state,
    assign: (assignee: string | null) =>
      record(
        { assignee },
        assignee ? `${actor} assigned the conversation to ${assignee}` : `${actor} unassigned the conversation`,
      ),
    setStatus: (status: Status) =>
      record({ status }, `${actor} marked the conversation ${statusLabel[status].toLowerCase()}`),
    setPriority: (priority: Priority) =>
      record({ priority }, `${actor} set the priority to ${priorityLabel[priority].toLowerCase()}`),
  };
}

export type CaseControls = ReturnType<typeof useCase>;
