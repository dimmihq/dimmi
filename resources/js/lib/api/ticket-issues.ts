/*
 * The engineering work behind tickets: issues and incidents linked from
 * Linear, Jira, GitHub, incident.io, Rootly and PagerDuty; the lists a
 * connected tool offers its pickers (teams, projects, states,
 * severities, services, users, fields, environments) and a search of
 * its issues; each tool's sync settings; the drafts that close the loop
 * when the work is done; and the workspace's ticket switches.
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

export type DraftStatus = "ready" | "sent" | "discarded";

export interface TicketUpdateDraft {
  object: "ticket_update_draft";
  id: string;
  ticket_id: string;
  issue_id: string | null;
  source: string;
  reason: "incident_resolved" | "comment";
  body: string;
  author_name: string;
  status: DraftStatus;
  resolve: boolean;
  update_id: string | null;
  recipients: number;
  held: Record<string, number>;
  decided_at: string | null;
  created_at: string;
}

export interface TicketSettings {
  object: "ticket_settings";
  auto_link_duplicates: boolean;
  spike_threshold: number;
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
  drafts: (workspace: string, ticket: string) => ["workspace", workspace, "ticket-drafts", ticket] as const,
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

export function issueOptionsQuery(workspace: string, installation: string, list: IssueList, parent = "") {
  const query = new URLSearchParams({ list });
  if (parent) {
    query.set("parent", parent);
  }

  return queryOptions({
    queryKey: issueKeys.options(workspace, installation, list, parent),
    queryFn: () =>
      request<IssueOption[]>(
        "GET",
        `/integration-installations/${enc(installation)}/issue-options?${query.toString()}`,
        undefined,
        { workspace },
      ),
    enabled: installation !== "",
    staleTime: 60_000,
  });
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

export function ticketDraftsQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: issueKeys.drafts(workspace, ticket),
    queryFn: () =>
      request<TicketUpdateDraft[]>("GET", `/tickets/${enc(ticket)}/update-drafts`, undefined, { workspace }),
    enabled: workspace !== "" && ticket !== "",
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

export function unlinkTicketIssue(workspace: string, issue: string) {
  return request<unknown>("DELETE", `/ticket-issues/${enc(issue)}`, undefined, { workspace });
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

export function decideDraft(
  workspace: string,
  draft: string,
  decision: { status: "sent" | "discarded"; body?: string; resolve?: boolean },
) {
  return request<TicketUpdateDraft>("PATCH", `/ticket-update-drafts/${enc(draft)}`, decision, { workspace });
}

export function updateTicketSettings(workspace: string, change: Partial<Omit<TicketSettings, "object">>) {
  return request<TicketSettings>("PATCH", "/ticket-settings", change, { workspace });
}

/*
 * refreshTicketKeys are what a link or a draft changes: the ticket's
 * issues, its drafts and the ticket itself.
 */
export function refreshTicketKeys(workspace: string, ticket: string) {
  return [issueKeys.ticket(workspace, ticket), issueKeys.drafts(workspace, ticket), ticketKeys.all(workspace)];
}
