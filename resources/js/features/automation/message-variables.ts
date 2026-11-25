/*
 * The details a workflow's message can name, filled in for each
 * customer as it goes out, and the teammates a note names.
 */
import type { EditorVariable } from "@/features/inbox/composer/rich-editor";

import type { TriggerKind } from "./triggers";

/*
 * baseVariables are the details every workflow message can name.
 */
const baseVariables: EditorVariable[] = [
  { key: "contact.first_name", label: "First name", detail: "The customer's first name" },
  { key: "contact.name", label: "Full name", detail: "The customer's full name" },
  { key: "contact.email", label: "Email", detail: "The customer's email address" },
  { key: "contact.phone", label: "Phone", detail: "The customer's phone number" },
  { key: "company.name", label: "Company", detail: "The customer's company" },
  { key: "conversation.number", label: "Conversation number", detail: "Such as 1042" },
  { key: "conversation.subject", label: "Subject", detail: "The conversation's subject" },
  { key: "workspace.name", label: "Workspace name", detail: "Your workspace's name" },
  { key: "schedule.next_open", label: "When you're back", detail: "When office hours open next" },
];

const handoverVariables: EditorVariable[] = [
  { key: "dimmi.summary", label: "Agent's summary", detail: "What the AI agent wrote when it handed over" },
  { key: "dimmi.handover_reason", label: "Handover reason", detail: "Why the AI agent handed over" },
];

const ticketVariables: EditorVariable[] = [
  { key: "ticket.reference", label: "Ticket reference", detail: "Such as T-1042" },
  { key: "ticket.title", label: "Ticket title", detail: "The ticket's title" },
];

/*
 * variablesFor lists the details a message under the trigger can name,
 * with the reply time for a reply time step.
 */
export function variablesFor(trigger: TriggerKind, replyTime = false): EditorVariable[] {
  const out = [...baseVariables];
  if (trigger.startsWith("dimmi.") || trigger.startsWith("agent.")) {
    out.push(...handoverVariables);
  }
  if (trigger.startsWith("ticket.") || trigger.startsWith("sla.")) {
    out.push(...ticketVariables);
  }
  if (replyTime) {
    out.unshift({
      key: "reply_time",
      label: "Reply time",
      detail: "Such as “within an hour” or “tomorrow after 09:00”",
    });
  }

  return out;
}

/*
 * mentionedIn lists the teammates a note names, in the <@id> form its
 * Markdown keeps them.
 */
export function mentionedIn(markdown: string): string[] {
  return [...new Set([...markdown.matchAll(/<@(mem_[a-z0-9]+)>/g)].map((match) => match[1] ?? ""))].filter(Boolean);
}

/*
 * readableText writes a message as a step's card shows it: each
 * variable as its name in braces, such as {First name}, each link as
 * its title, each mention as an at sign, and bold as plain words.
 */
export function readableText(markdown: string): string {
  const all = [...baseVariables, ...handoverVariables, ...ticketVariables];

  return markdown
    .replace(/\{\{\s*([a-z][a-z0-9_.]*)\s*(\|[^}]*)?\}\}/g, (_, key: string) => {
      const label = key === "reply_time" ? "Reply time" : all.find((v) => v.key === key)?.label;

      return `{${label ?? key}}`;
    })
    .replace(/\[([^\]\n]+)\]\([^)\s]+\)/g, "$1")
    .replace(/<@[a-z]+_[a-z0-9]+>/g, "@teammate")
    .replace(/(\*\*|__)(.+?)\1/g, "$2")
    .replace(/\\([\\`*_{}[\]()#+\-.!>])/g, "$1");
}
