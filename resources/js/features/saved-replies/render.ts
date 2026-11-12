/*
 * Filling a saved reply's variables for one conversation. A reply keeps
 * them the way broadcasts do, in Liquid's form with a fallback:
 *
 *	{{ contact.first_name | default: "there" }}
 *
 * and they are written out when the reply is inserted, so the teammate
 * sees and can still edit exactly what the customer will read. A detail
 * the conversation does not know takes its fallback.
 */
import { variablePattern } from "@/features/broadcasts/writer/variables";

const anywhere = new RegExp(variablePattern.source.slice(1), "g");

/*
 * ReplyContext is what a reply's variables can name: the customer and
 * the teammate sending it.
 */
export interface ReplyContext {
  contact?: { name?: string | null; email?: string | null } | null;
  company?: string | null;
  sender?: string | null;
}

function firstWord(name: string | null | undefined): string {
  return name?.trim().split(/\s+/)[0] ?? "";
}

/*
 * contextValues lists each variable's value for the context, empty when
 * it is not known.
 */
export function contextValues(context: ReplyContext): Record<string, string> {
  const name = context.contact?.name?.trim() ?? "";

  return {
    "contact.first_name": firstWord(name),
    "contact.name": name,
    "contact.email": context.contact?.email ?? "",
    "company.name": context.company ?? "",
    "sender.first_name": firstWord(context.sender),
    "sender.name": context.sender ?? "",
  };
}

/*
 * renderReply writes every variable in the Markdown as its value for the
 * context, or its fallback when the value is not known.
 */
export function renderReply(markdown: string, context: ReplyContext): string {
  const values = contextValues(context);

  return markdown.replace(anywhere, (_, key: string, fallback?: string) => values[key] || fallback || "");
}
