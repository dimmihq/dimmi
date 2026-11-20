/*
 * The engineering tools a ticket links to, in plain words: what each is
 * called, whether it holds issues or incidents, what its lists are
 * called in its own terms (a Jira project is where a Linear team would
 * be), and the dimmi state a status maps to when nobody picked one. The
 * defaults mirror the server's, for issues and incidents alike: started
 * is "Being worked on", done closes the loop, canceled is "Won't fix".
 */
import type { IssueOption } from "@/lib/api/ticket-issues";
import type { TicketState } from "@/lib/api/tickets";

export type ToolKind = "issues" | "incidents";

export interface ToolLook {
  label: string;
  kind: ToolKind;
  team?: string;
  project?: string;
  search: string;
}

const looks: Record<string, ToolLook> = {
  linear: { label: "Linear", kind: "issues", team: "Team", project: "Project", search: "Search Linear or paste a URL" },
  jira: { label: "Jira", kind: "issues", team: "Project", project: "Issue type", search: "Search Jira or paste a key" },
  github: { label: "GitHub", kind: "issues", team: "Repository", search: "Search GitHub or paste a URL" },
  incident_io: { label: "incident.io", kind: "incidents", search: "Search incident.io incidents" },
  rootly: { label: "Rootly", kind: "incidents", search: "Search Rootly incidents" },
  pagerduty: { label: "PagerDuty", kind: "incidents", search: "Search open PagerDuty incidents" },
};

export function isTool(provider: string): boolean {
  return provider in looks;
}

export function toolLook(provider: string): ToolLook {
  return looks[provider] ?? { label: provider, kind: "issues", search: "Search" };
}

/*
 * toolByLabel names the tool behind a label, such as the "Linear" a
 * comment synced from Linear is written as, or undefined for anyone
 * else.
 */
export function toolByLabel(label: string): string | undefined {
  return Object.keys(looks).find((provider) => looks[provider]?.label === label);
}

/*
 * groupTint colours a status by its group. Backlog and unstarted take
 * the tertiary tint so they stand out on the secondary cards they sit
 * in.
 */
export const groupTint: Record<string, "neutral" | "blue" | "lime" | "rose" | "yellow"> = {
  backlog: "neutral",
  unstarted: "neutral",
  started: "blue",
  done: "lime",
  canceled: "rose",
};

/*
 * defaultState is the state a status moves a ticket to when the
 * workspace picked none, or undefined when it leaves the ticket be.
 */
export function defaultState(status: IssueOption, states: TicketState[]): TicketState | undefined {
  const purpose = { started: "being_worked_on", done: "close_loop", canceled: "wont_fix" }[status.group];

  return purpose ? states.find((s) => s.purpose === purpose && !s.archived_at) : undefined;
}

/*
 * written says what dimmi keeps on an issue in each tool that it can
 * take back on unlinking, or undefined for a tool it cannot.
 */
const written: Record<string, string> = {
  linear: "The impact attachment linking back to this ticket.",
  jira: "The web link back to this ticket and the customer impact property.",
  incident_io: "The customer count dimmi keeps in the custom field.",
  rootly: "The dimmi_ labels holding the customer impact and the link back.",
};

export function writtenOn(provider: string): string | undefined {
  return written[provider];
}
