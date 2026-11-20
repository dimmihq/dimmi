/*
 * The engineering work behind tickets: issues and incidents linked from
 * Linear, Jira, GitHub, incident.io, Rootly and PagerDuty; the lists a
 * connected tool offers its pickers (teams, projects, states,
 * severities, services, users, fields, environments) and a search of
 * its issues; each tool's sync settings; and the workspace's ticket
 * switches.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { Integration } from "./integrations";
import { type TicketIssue, ticketKeys } from "./tickets";

export type IssueList =
  | "teams"
  | "projects"
  | "states"
  | "severities"
  | "services"
  | "users"
  | "fields"
  | "environments";

export interface IssueOption {
  object: "issue_option";
  id: string;
  name: string;
  detail: string;
  group: string;
  rank: number;
}

export interface IssueFound {
  object: "issue_found";
  external_id: string;
  key: string;
  url: string;
  title: string;
  status: string;
  status_group: string;
  team_name: string;
  severity: string;
}

export interface IssueSettings {
  object: "issue_settings";
  installation_id: string;
  provider: string;
  states: Record<string, string>;
  team_id: string;
  project_id: string;
  service_id: string;
  severity_id: string;
  from_email: string;
  impact_field_id: string;
  deploy_repository: string;
  deploy_environment: string;
  webhook_url: string;
  needs_signing_secret: boolean;
  has_signing_secret: boolean;
}

export type IssueSettingsChange = Partial<
  Omit<
    IssueSettings,
    "object" | "installation_id" | "provider" | "webhook_url" | "needs_signing_secret" | "has_signing_secret"
  >
> & { signing_secret?: string };

export interface TicketSettings {
  object: "ticket_settings";
  auto_link_duplicates: boolean;
}

/*
 * NewIssue creates an issue in a tool, or links one by its URL or key.
 */
export interface NewIssue {
  installation_id?: string;
  team_id?: string;
  project_id?: string;
  service_id?: string;
  severity_id?: string;
  title?: string;
  url?: string;
  key?: string;
}

export const issueKeys = {
  ticket: (workspace: string, ticket: string) => ["workspace", workspace, "ticket-issues", ticket] as const,
  options: (workspace: string, installation: string, list: IssueList, parent: string) =>
    ["workspace", workspace, "issue-options", installation, list, parent] as const,
  search: (workspace: string, installation: string, q: string) =>
    ["workspace", workspace, "issue-search", installation, q] as const,
  settings: (workspace: string, installation: string) =>
    ["workspace", workspace, "issue-settings", installation] as const,
  ticketSettings: (workspace: string) => ["workspace", workspace, "ticket-settings"] as const,
};

const enc = encodeURIComponent;

export function ticketIssuesQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: issueKeys.ticket(workspace, ticket),
    queryFn: () => request<TicketIssue[]>("GET", `/tickets/${enc(ticket)}/issues`, undefined, { workspace }),
    enabled: workspace !== "" && ticket !== "",
  });
}

/*
 * readIssueOptions reads one of a tool's lists. The server keeps each
 * list for a day; fresh pulls it from the tool again.
 */
function readIssueOptions(workspace: string, installation: string, list: IssueList, parent: string, fresh: boolean) {
  const query = new URLSearchParams({ list });
  if (parent) {
    query.set("parent", parent);
  }
  if (fresh) {
    query.set("refresh", "true");
  }

  return request<IssueOption[]>(
    "GET",
    `/integration-installations/${enc(installation)}/issue-options?${query.toString()}`,
    undefined,
    { workspace },
  );
}

export function issueOptionsQuery(workspace: string, installation: string, list: IssueList, parent = "") {
  return queryOptions({
    queryKey: issueKeys.options(workspace, installation, list, parent),
    queryFn: () => readIssueOptions(workspace, installation, list, parent, false),
    enabled: installation !== "",
    staleTime: 5 * 60_000,
  });
}

/*
 * refreshIssueOptions pulls one of a tool's lists from the tool again,
 * such as its teams after one was added there.
 */
export function refreshIssueOptions(workspace: string, installation: string, list: IssueList, parent = "") {
  return readIssueOptions(workspace, installation, list, parent, true);
}

export function issueSearchQuery(workspace: string, installation: string, q: string) {
  return queryOptions({
    queryKey: issueKeys.search(workspace, installation, q),
    queryFn: () =>
      request<IssueFound[]>("GET", `/integration-installations/${enc(installation)}/issues?q=${enc(q)}`, undefined, {
        workspace,
      }),
    enabled: installation !== "" && q.trim().length > 1,
  });
}

export function issueSettingsQuery(workspace: string, installation: string) {
  return queryOptions({
    queryKey: issueKeys.settings(workspace, installation),
    queryFn: () =>
      request<IssueSettings>("GET", `/integration-installations/${enc(installation)}/issue-settings`, undefined, {
        workspace,
      }),
  });
}

export function ticketSettingsQuery(workspace: string) {
  return queryOptions({
    queryKey: issueKeys.ticketSettings(workspace),
    queryFn: () => request<TicketSettings>("GET", "/ticket-settings", undefined, { workspace }),
  });
}

export function linkTicketIssue(workspace: string, ticket: string, issue: NewIssue) {
  return request<TicketIssue>("POST", `/tickets/${enc(ticket)}/issues`, issue, { workspace });
}

/*
 * refreshTicketIssues reads a ticket's linked issues from their tools
 * now, instead of waiting for the next delivery or re-check.
 */
export function refreshTicketIssues(workspace: string, ticket: string) {
  return request<TicketIssue[]>("POST", `/tickets/${enc(ticket)}/issues/refresh`, undefined, { workspace });
}

/*
 * unlinkTicketIssue unlinks an issue. removeFromTool also takes back what
 * dimmi wrote on it in its tool; otherwise the tool keeps it.
 */
export function unlinkTicketIssue(workspace: string, issue: string, removeFromTool = false) {
  const query = removeFromTool ? "?remove_from_tool=true" : "";

  return request<unknown>("DELETE", `/ticket-issues/${enc(issue)}${query}`, undefined, { workspace });
}

export function updateIssueSettings(workspace: string, installation: string, change: IssueSettingsChange) {
  return request<IssueSettings>("PATCH", `/integration-installations/${enc(installation)}/issue-settings`, change, {
    workspace,
  });
}

export function connectWithKey(workspace: string, provider: string, apiKey: string) {
  return request<Integration>(
    "POST",
    `/integrations/${enc(provider)}/key-installations`,
    { api_key: apiKey },
    {
      workspace,
    },
  );
}

export function updateTicketSettings(workspace: string, change: Partial<Omit<TicketSettings, "object">>) {
  return request<TicketSettings>("PATCH", "/ticket-settings", change, { workspace });
}

/*
 * refreshTicketKeys are what a link changes: the ticket's issues and the
 * ticket itself.
 */
export function refreshTicketKeys(workspace: string, ticket: string) {
  return [issueKeys.ticket(workspace, ticket), ticketKeys.all(workspace)];
}
