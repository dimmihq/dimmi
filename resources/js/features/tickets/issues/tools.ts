/*
 * The engineering tools a ticket links to, in plain words: what each is
 * called, whether it holds issues or incidents, what its lists are
 * called in its own terms (a Jira project is where a Linear team would
 * be), and the dimmi state a status maps to when nobody picked one. The
 * defaults mirror the server's: started is "Being worked on", done
 * closes the loop, canceled is "Won't fix"; an incident follows its
 * tool from investigating to resolved.
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
 * groupTint colours a status by its group.
 */
export const groupTint: Record<string, "gray" | "blue" | "lime" | "rose" | "yellow"> = {
  backlog: "gray",
  unstarted: "gray",
  started: "blue",
  done: "lime",
  canceled: "rose",
};

/*
 * defaultState is the state a status moves a ticket to when the
 * workspace picked none, or undefined when it leaves the ticket be.
 */
export function defaultState(kind: ToolKind, status: IssueOption, states: TicketState[]): TicketState | undefined {
  const byPurpose = (purpose: string) => states.find((s) => s.purpose === purpose && !s.archived_at);

  if (kind === "issues") {
    const purpose = { started: "being_worked_on", done: "close_loop", canceled: "wont_fix" }[status.group];
    return purpose ? byPurpose(purpose) : undefined;
  }

  const name = status.name.toLowerCase();
  if (status.group === "done" || status.group === "canceled") {
    return states.find((s) => s.group === "resolved" && !s.purpose && !s.archived_at);
  }
  if (name.includes("monitor") || name.includes("mitigat")) {
    return byPurpose("incident_monitoring");
  }
  if (name.includes("identif") || name.includes("fix") || name.includes("acknowledg")) {
    return byPurpose("incident_identified");
  }

  return byPurpose("investigating");
}

/*
 * draftReasons say why a draft was written.
 */
export const draftReasons: Record<string, string> = {
  incident_resolved: "The incident is resolved",
  comment: "An engineer wrote to customers",
};

/*
 * heldReasons explain why a customer would be held back.
 */
export const heldReasons: Record<string, string> = {
  customer_wrote_since: "wrote since",
  teammate_already_replied: "already told",
};
