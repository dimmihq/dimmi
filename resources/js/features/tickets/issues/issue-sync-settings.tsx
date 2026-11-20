/*
 * The settings of a connected engineering tool, on its integration
 * page. Where the tool delivers, with the signing secret a key-connected
 * tool needs pasted; the team, project, service and severity new work
 * goes to; which dimmi state each of the tool's statuses moves a ticket
 * to, defaulting to Being worked on, Close the loop and Won't fix; for
 * GitHub, the deployments that prove a fix shipped. Every choice is
 * picked from lists pulled from the tool. Incidents are only linked by
 * hand from a ticket: nothing opens in dimmi from the tool by itself.
 * Every pick and the pasted secret wait in the page's draft until its
 * save bar sends them.
 */
import { RiFileCopyLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { Row, Section } from "@/features/settings/layout";
import type { Integration } from "@/lib/api/integrations";
import {
  type IssueList,
  type IssueSettings,
  type IssueSettingsChange,
  issueKeys,
  issueOptionsQuery,
  refreshIssueOptions,
} from "@/lib/api/ticket-issues";
import { ticketStatesQuery } from "@/lib/api/tickets";

import type { IssueSyncDraft } from "./issue-sync-draft";
import { QuietRefresh, StatusRow, TeamStatuses } from "./team-statuses";
import { isTool, type ToolKind, toolLook } from "./tools";

type Save = (change: IssueSettingsChange) => void;

/*
 * OptionSelect is a compact select fed by one of the tool's lists.
 */
function OptionSelect({
  workspace,
  installation,
  list,
  parent = "",
  label,
  value,
  disabled,
  onChange,
}: {
  workspace: string;
  installation: string;
  list: IssueList;
  parent?: string;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (id: string) => void;
}) {
  const { data, isPending, isError } = useQuery(issueOptionsQuery(workspace, installation, list, parent));

  return (
    <Select
      aria-label={label}
      size="sm"
      className="w-60"
      placeholder={isError ? "Could not load the list" : isPending ? "Loading…" : "Pick one"}
      isDisabled={disabled || isPending || isError}
      selectedKey={value || null}
      onSelectionChange={(key) => onChange(key ? String(key) : "")}
    >
      {(data ?? []).map((o) => (
        <SelectItem key={o.id} id={o.id} textValue={o.name}>
          {o.name}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * useRefreshLists pulls lists of the tool again and puts them where
 * the pickers read them.
 */
function useRefreshLists(workspace: string, installation: string, provider: string, lists: IssueList[]) {
  const client = useQueryClient();
  const label = toolLook(provider).label;

  return useMutation({
    mutationFn: () => Promise.all(lists.map((list) => refreshIssueOptions(workspace, installation, list))),
    onSuccess: (fresh) => {
      lists.forEach((list, i) => {
        client.setQueryData(issueKeys.options(workspace, installation, list, ""), fresh[i]);
      });
      toast.success(`Read again from ${label}.`);
    },
    onError: (error) => toast.error((error as Error).message),
  });
}

/*
 * webhookSteps say where in each key-connected tool its webhook is made
 * and which events to send.
 */
const webhookSteps: Record<string, string> = {
  incident_io:
    "In incident.io, open Settings, Webhooks and add an endpoint with the address below. Subscribe to public_incident.incident_created_v2, incident_updated_v2 and incident_status_updated_v2.",
  rootly:
    "In Rootly, open Configuration, Webhooks and add an endpoint with the address below. Subscribe to the incident events and incident_event.created.",
  pagerduty: "In PagerDuty, open Integrations, Generic Webhooks (v3) and add a subscription with the address below.",
};

function DeliverySection({
  settings,
  secret,
  canManage,
  save,
}: {
  settings: IssueSettings;
  secret: string;
  canManage: boolean;
  save: Save;
}) {
  const appWide = settings.provider === "linear" || settings.provider === "jira";
  const label = toolLook(settings.provider).label;
  const steps = settings.needs_signing_secret ? webhookSteps[settings.provider] : undefined;

  return (
    <Section
      title="Deliveries"
      description={
        steps
          ? `${steps} Changes are also re-checked every 15 minutes.`
          : "Changes in the tool reach dimmi here, and are re-checked every 15 minutes."
      }
    >
      <Row
        label={steps ? "1. Webhook address" : "Webhook address"}
        description={
          appWide
            ? "dimmi's app delivers here; nothing to set up."
            : steps
              ? `Where ${label} sends incident changes.`
              : "Create a webhook in the tool pointing here."
        }
      >
        <span className="flex items-center gap-2">
          <code className="max-w-72 truncate text-caption-1-regular text-text-secondary">{settings.webhook_url}</code>
          <IconButton
            size="small"
            icon={RiFileCopyLine}
            aria-label="Copy the webhook address"
            onClick={() => {
              void navigator.clipboard?.writeText(settings.webhook_url);
              toast.success("Copied.");
            }}
          />
        </span>
      </Row>
      {settings.needs_signing_secret ? (
        <Row
          label="2. Signing secret"
          description={
            settings.has_signing_secret
              ? "Set. Paste a new one to replace it."
              : `Copy it from the endpoint you added in ${label}. Deliveries are refused until it is set.`
          }
        >
          <span className="flex items-center gap-2">
            {settings.has_signing_secret ? (
              <Chip variant="caption" color="lime">
                Set
              </Chip>
            ) : null}
            <Input
              aria-label="Signing secret"
              type="password"
              size="small"
              className="w-48"
              value={secret}
              isDisabled={!canManage}
              onChange={(text) => save({ signing_secret: text })}
            />
          </span>
        </Row>
      ) : null}
    </Section>
  );
}

/*
 * defaultLists are the lists the defaults of an incident tool pick from.
 */
function defaultLists(provider: string, kind: ToolKind): IssueList[] {
  if (kind !== "incidents") {
    return [];
  }

  return (
    {
      incident_io: ["severities", "fields"] as IssueList[],
      rootly: ["severities", "services"] as IssueList[],
      pagerduty: ["services", "users", "severities"] as IssueList[],
    }[provider] ?? ["severities"]
  );
}

function DefaultsSection({
  workspace,
  settings,
  kind,
  canManage,
  save,
}: {
  workspace: string;
  settings: IssueSettings;
  kind: ToolKind;
  canManage: boolean;
  save: Save;
}) {
  const look = toolLook(settings.provider);
  const id = settings.installation_id;
  const disabled = !canManage;
  const lists = defaultLists(settings.provider, kind);
  const refresh = useRefreshLists(workspace, id, settings.provider, lists);

  return (
    <Section
      title={kind === "incidents" ? "Declaring from dimmi" : "New issues"}
      description={kind === "incidents" ? "Picked first when declaring an incident from a ticket." : undefined}
      action={
        kind === "incidents" ? (
          <QuietRefresh label="Refresh" busy={refresh.isPending} onPress={() => refresh.mutate()} />
        ) : undefined
      }
    >
      {kind === "issues" ? (
        <Row
          label={`Default ${(look.team ?? "team").toLowerCase()}`}
          description="Picked first when creating an issue."
        >
          <OptionSelect
            workspace={workspace}
            installation={id}
            list="teams"
            label={look.team ?? "Team"}
            value={settings.team_id}
            disabled={disabled}
            onChange={(team) => save({ team_id: team, project_id: "" })}
          />
        </Row>
      ) : null}
      {kind === "issues" && look.project && settings.team_id ? (
        <Row label={`Default ${look.project.toLowerCase()}`}>
          <OptionSelect
            workspace={workspace}
            installation={id}
            list="projects"
            parent={settings.team_id}
            label={look.project}
            value={settings.project_id}
            disabled={disabled}
            onChange={(project) => save({ project_id: project })}
          />
        </Row>
      ) : null}
      {settings.provider === "pagerduty" ? (
        <>
          <Row label="Service to page" description="Declaring triggers an incident here, paging whoever is on call.">
            <OptionSelect
              workspace={workspace}
              installation={id}
              list="services"
              label="Service"
              value={settings.service_id}
              disabled={disabled}
              onChange={(service) => save({ service_id: service })}
            />
          </Row>
          <Row label="Act as" description="PagerDuty records who triggered and noted each incident.">
            <OptionSelect
              workspace={workspace}
              installation={id}
              list="users"
              label="PagerDuty user"
              value={settings.from_email}
              disabled={disabled}
              onChange={(email) => save({ from_email: email })}
            />
          </Row>
        </>
      ) : null}
      {settings.provider === "rootly" ? (
        <Row label="Default service" description="The service a declared incident affects.">
          <OptionSelect
            workspace={workspace}
            installation={id}
            list="services"
            label="Service"
            value={settings.service_id}
            disabled={disabled}
            onChange={(service) => save({ service_id: service })}
          />
        </Row>
      ) : null}
      {kind === "incidents" ? (
        <Row label="Default severity">
          <OptionSelect
            workspace={workspace}
            installation={id}
            list="severities"
            label="Severity"
            value={settings.severity_id}
            disabled={disabled}
            onChange={(severity) => save({ severity_id: severity })}
          />
        </Row>
      ) : null}
      {settings.provider === "incident_io" ? (
        <Row
          label="Customer count field"
          description="A number custom field dimmi keeps at the customers a ticket reaches."
        >
          <OptionSelect
            workspace={workspace}
            installation={id}
            list="fields"
            label="Custom field"
            value={settings.impact_field_id}
            disabled={disabled}
            onChange={(field) => save({ impact_field_id: field })}
          />
        </Row>
      ) : null}
    </Section>
  );
}

/*
 * StatusSection maps the tool's statuses to dimmi states: team by team
 * for a tool whose workflows belong to its teams, else in one list.
 */
function StatusSection({
  workspace,
  settings,
  canManage,
  save,
}: {
  workspace: string;
  settings: IssueSettings;
  canManage: boolean;
  save: Save;
}) {
  const byTeam = settings.provider === "linear" || settings.provider === "jira";
  const { data: statuses } = useQuery({
    ...issueOptionsQuery(workspace, settings.installation_id, "states", ""),
    enabled: !byTeam,
  });
  const { data: states } = useQuery(ticketStatesQuery(workspace));
  const live = (states ?? []).filter((s) => !s.archived_at);
  const refresh = useRefreshLists(workspace, settings.installation_id, settings.provider, ["states"]);

  if (byTeam) {
    return <TeamStatuses workspace={workspace} settings={settings} canManage={canManage} save={save} />;
  }

  return (
    <Section
      title="Statuses"
      description="Customers only ever see dimmi's states. Each status of the tool moves a linked ticket here."
      action={<QuietRefresh label="Refresh" busy={refresh.isPending} onPress={() => refresh.mutate()} />}
    >
      {(statuses ?? []).map((status) => (
        <StatusRow
          key={status.id}
          status={status}
          picked={settings.states[status.id] ?? ""}
          states={live}
          disabled={!canManage}
          onPick={(state) => {
            const next = { ...settings.states };
            if (state) {
              next[status.id] = state;
            } else {
              delete next[status.id];
            }
            save({ states: next });
          }}
        />
      ))}
    </Section>
  );
}

function DeploySection({
  workspace,
  settings,
  canManage,
  save,
}: {
  workspace: string;
  settings: IssueSettings;
  canManage: boolean;
  save: Save;
}) {
  return (
    <Section
      title="Deployments"
      description="A successful deployment marks fixes done before it as shipped, for workflows that close the loop."
    >
      <Row label="Repository">
        <OptionSelect
          workspace={workspace}
          installation={settings.installation_id}
          list="teams"
          label="Repository"
          value={settings.deploy_repository}
          disabled={!canManage}
          onChange={(repo) => save({ deploy_repository: repo, deploy_environment: "" })}
        />
      </Row>
      {settings.deploy_repository ? (
        <Row label="Environment">
          <OptionSelect
            workspace={workspace}
            installation={settings.installation_id}
            list="environments"
            parent={settings.deploy_repository}
            label="Environment"
            value={settings.deploy_environment}
            disabled={!canManage}
            onChange={(env) => save({ deploy_environment: env })}
          />
        </Row>
      ) : null}
    </Section>
  );
}

export function IssueSyncSettings({
  workspace,
  integration,
  canManage,
  draft,
}: {
  workspace: string;
  integration: Integration;
  canManage: boolean;
  draft: IssueSyncDraft;
}) {
  const { settings } = draft;
  const kind = toolLook(integration.provider).kind;

  if (!settings || !isTool(integration.provider) || integration.status !== "active") {
    return null;
  }

  const props = { workspace, settings, canManage, save: draft.set };

  return (
    <>
      <DeliverySection settings={settings} secret={draft.secret} canManage={canManage} save={props.save} />
      <DefaultsSection {...props} kind={kind} />
      <StatusSection {...props} />
      {integration.provider === "github" ? <DeploySection {...props} /> : null}
    </>
  );
}
