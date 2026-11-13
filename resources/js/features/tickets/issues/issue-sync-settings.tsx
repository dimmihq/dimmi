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
import { useState } from "react";
import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { Row, Section } from "@/features/settings/layout";
import type { Integration } from "@/lib/api/integrations";
import {
  type IssueList,
  type IssueOption,
  type IssueSettings,
  type IssueSettingsChange,
  issueKeys,
  issueOptionsQuery,
  issueSettingsQuery,
  updateIssueSettings,
} from "@/lib/api/ticket-issues";
import { ticketStatesQuery } from "@/lib/api/tickets";

import { defaultState, groupTint, isTool, type ToolKind, toolLook } from "./tools";

type Save = (change: IssueSettingsChange) => void;

/*
 * useIssueSyncDraft holds the tool's settings changed but not saved yet,
 * laid over them as saved. A change back to the saved value drops out,
 * and so does an empty secret. Save sends the rest in one update.
 */
export function useIssueSyncDraft(workspace: string, installation: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const { data: saved } = useQuery({ ...issueSettingsQuery(workspace, installation), enabled });
  const [pending, setPending] = useState<IssueSettingsChange>({});
  const save = useMutation({
    mutationFn: () =>
      updateIssueSettings(workspace, installation, {
        ...pending,
        ...(pending.signing_secret ? { signing_secret: pending.signing_secret.trim() } : {}),
      }),
    onSuccess: (next) => {
      queryClient.setQueryData(issueKeys.settings(workspace, installation), next);
      setPending({});
      toast.success("Sync settings saved");
    },
  });
  const { signing_secret: secret = "", ...picked } = pending;

  return {
    settings: saved && { ...saved, ...picked },
    secret,
    dirty: Object.keys(pending).length > 0,
    save,
    set: (change: IssueSettingsChange) =>
      setPending((before) => {
        const next: Record<string, unknown> = { ...before, ...change };
        for (const [key, value] of Object.entries(change)) {
          const same =
            key === "signing_secret"
              ? String(value).trim() === ""
              : JSON.stringify(value) === JSON.stringify(saved?.[key as keyof IssueSettings]);
          if (same) {
            delete next[key];
          }
        }

        return next as IssueSettingsChange;
      }),
    discard: () => {
      save.reset();
      setPending({});
    },
  };
}

export type IssueSyncDraft = ReturnType<typeof useIssueSyncDraft>;

/*
 * defaultKey is the option that leaves a status on its default state.
 */
const defaultKey = "default";

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

  return (
    <Section
      title="Deliveries"
      description="Changes in the tool reach dimmi here, and are re-checked every 15 minutes."
    >
      <Row
        label="Webhook address"
        description={
          appWide ? "dimmi's app delivers here; nothing to set up." : "Create a webhook in the tool pointing here."
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
          label="Signing secret"
          description={
            settings.has_signing_secret
              ? "Set. Paste a new one to replace it."
              : "Paste the secret the tool shows for the webhook. Deliveries are refused until it is set."
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

  return (
    <Section title={kind === "incidents" ? "Declaring from dimmi" : "New issues"}>
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
        <Row label="Customer count field" description="A numeric custom field kept at the customers reached.">
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

function StatusRow({
  status,
  picked,
  fallback,
  states,
  disabled,
  onPick,
}: {
  status: IssueOption;
  picked: string;
  fallback: string;
  states: { id: string; name: string }[];
  disabled: boolean;
  onPick: (state: string) => void;
}) {
  return (
    <Row
      label={
        <span className="flex items-center gap-2">
          {status.name}
          <Chip variant="caption" color={groupTint[status.group] ?? "gray"}>
            {status.group || "status"}
          </Chip>
        </span>
      }
    >
      <Select
        aria-label={`State for ${status.name}`}
        size="sm"
        className="w-60"
        isDisabled={disabled}
        selectedKey={picked || defaultKey}
        onSelectionChange={(key) => onPick(key && key !== defaultKey ? String(key) : "")}
      >
        <SelectItem id={defaultKey} textValue={`Default: ${fallback}`}>
          {`Default: ${fallback}`}
        </SelectItem>
        {states.map((s) => (
          <SelectItem key={s.id} id={s.id} textValue={s.name}>
            {s.name}
          </SelectItem>
        ))}
      </Select>
    </Row>
  );
}

function StatusSection({
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
  const parent = settings.provider === "linear" || settings.provider === "jira" ? settings.team_id : "";
  const needsParent = parent === "" && (settings.provider === "linear" || settings.provider === "jira");
  const { data: statuses } = useQuery({
    ...issueOptionsQuery(workspace, settings.installation_id, "states", parent),
    enabled: !needsParent,
  });
  const { data: states } = useQuery(ticketStatesQuery(workspace));
  const live = (states ?? []).filter((s) => !s.archived_at);

  return (
    <Section
      title="Statuses"
      description="Customers only ever see dimmi's states. Each status of the tool moves a linked ticket here."
    >
      {needsParent ? (
        <Row label="Pick a default team first" description="Statuses are read from that team's workflow." muted />
      ) : null}
      {(statuses ?? []).map((status) => (
        <StatusRow
          key={status.id}
          status={status}
          picked={settings.states[status.id] ?? ""}
          fallback={defaultState(kind, status, live)?.name ?? "Leave as is"}
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
      <StatusSection {...props} kind={kind} />
      {integration.provider === "github" ? <DeploySection {...props} /> : null}
    </>
  );
}
