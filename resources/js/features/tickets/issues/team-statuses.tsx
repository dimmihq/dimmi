/*
 * The statuses of a tool whose workflows belong to its teams, such as
 * Linear's teams or Jira's projects. Each team is a row saying how many
 * of its statuses have a state picked; opening one shows its statuses,
 * each moving a linked ticket to the dimmi state picked, or to the
 * standard one: started is Being worked on, done closes the loop,
 * canceled is Won't fix. Teams and statuses come from the tool and the
 * server keeps them for a day; the refresh buttons pull them again,
 * as after adding a team in the tool. Picks wait in the page's draft
 * until its save bar sends them.
 */

import { RiArrowRightSLine, RiRefreshLine } from "@remixicon/react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { toast } from "@/components/base/toast/toast";
import { Row, RowsSkeleton, Section } from "@/features/settings/layout";
import {
  type IssueOption,
  type IssueSettings,
  type IssueSettingsChange,
  issueKeys,
  issueOptionsQuery,
  refreshIssueOptions,
} from "@/lib/api/ticket-issues";
import { type TicketState, ticketStatesQuery } from "@/lib/api/tickets";
import { countOf } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { ToolTile } from "./tool-logos";
import { defaultState, groupTint, toolLook } from "./tools";

type Save = (change: IssueSettingsChange) => void;

/*
 * defaultKey is the option that leaves a status on its standard state.
 */
const defaultKey = "default";

/*
 * teamWords name a tool's teams in its own terms.
 */
const teamWords: Record<string, { one: string; many: string }> = {
  linear: { one: "team", many: "teams" },
  jira: { one: "project", many: "projects" },
};

function teamWordsOf(provider: string) {
  return teamWords[provider] ?? { one: "team", many: "teams" };
}

/*
 * pickStates returns the picks with one status moved to a state, or back
 * to its standard one when state is empty.
 */
function pickStates(picks: Record<string, string>, status: string, state: string) {
  const next = { ...picks };
  if (state) {
    next[status] = state;
  } else {
    delete next[status];
  }

  return next;
}

/*
 * QuietRefresh is a small text action that reads a list from the tool
 * again; its arrows turn while it works.
 */
function Turning({ className }: { className?: string }) {
  return <RiRefreshLine aria-hidden className={cx(className, "animate-spin")} />;
}

export function QuietRefresh({ label, busy, onPress }: { label: string; busy: boolean; onPress: () => void }) {
  return (
    <LinkButton
      size="xs"
      variant="secondary"
      leadingIcon={busy ? Turning : RiRefreshLine}
      disabled={busy}
      onClick={onPress}
    >
      {label}
    </LinkButton>
  );
}

export function StatusRow({
  status,
  picked,
  states,
  disabled,
  onPick,
}: {
  status: IssueOption;
  picked: string;
  states: TicketState[];
  disabled: boolean;
  onPick: (state: string) => void;
}) {
  const fallback = defaultState(status, states)?.name ?? "Leave as is";

  return (
    <Row
      label={
        <span className="flex items-center gap-2">
          {status.name}
          <Chip variant="caption" color={groupTint[status.group] ?? "neutral"}>
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
        <SelectItem id={defaultKey} textValue={`Standard: ${fallback}`}>
          {`Standard: ${fallback}`}
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

/*
 * mappedLine says how a team's statuses map: how many have a state
 * picked, or that all use the standard states.
 */
function mappedLine(statuses: IssueOption[] | undefined, picks: Record<string, string>, failed: boolean) {
  if (failed) {
    return "Could not read its statuses";
  }
  if (!statuses) {
    return null;
  }
  const picked = statuses.filter((s) => picks[s.id]).length;
  if (picked === 0) {
    return `${countOf(statuses.length, "status", "statuses")}, all standard`;
  }

  return `${picked} of ${statuses.length} statuses picked`;
}

function TeamRow({
  provider,
  team,
  detail,
  isDefault,
  onOpen,
}: {
  provider: string;
  team: IssueOption;
  detail: string | null;
  isDefault: boolean;
  onOpen: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="-ms-1.5 flex min-h-[52px] w-[calc(100%+0.375rem)] cursor-pointer items-center gap-3 rounded-lg border-b border-separator-border py-2.5 ps-1.5 pe-2.5 text-start outline-none transition-colors duration-150 last:border-b-0 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <ToolTile provider={provider} size="sm" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="flex items-center gap-2 text-body-regular text-text-primary">
          <span className="truncate">{team.name}</span>
          {isDefault ? (
            <Chip variant="caption" color="neutral" className="py-0">
              Default
            </Chip>
          ) : null}
        </span>
        {detail ? (
          <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
        ) : (
          <Skeleton shape="text" className="mt-1 h-3 w-36" />
        )}
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

function StatusesModal({
  workspace,
  settings,
  team,
  canManage,
  save,
  onClose,
}: {
  workspace: string;
  settings: IssueSettings;
  team: IssueOption;
  canManage: boolean;
  save: Save;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const installation = settings.installation_id;
  const look = toolLook(settings.provider);
  const {
    data: statuses,
    isPending,
    isError,
  } = useQuery(issueOptionsQuery(workspace, installation, "states", team.id));
  const { data: states } = useQuery(ticketStatesQuery(workspace));
  const live = (states ?? []).filter((s) => !s.archived_at);
  const refresh = useMutation({
    mutationFn: () => refreshIssueOptions(workspace, installation, "states", team.id),
    onSuccess: (fresh) => {
      client.setQueryData(issueKeys.options(workspace, installation, "states", team.id), fresh);
      toast.success(`Statuses read again from ${look.label}.`);
    },
    onError: (error) => toast.error((error as Error).message),
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`${team.name} statuses`}
        description={`Each ${look.label} status moves a linked ticket to the dimmi state picked. Customers only ever see dimmi's states.`}
      />
      <ModalBody>
        <div
          aria-busy={isPending || undefined}
          className="flex flex-col rounded-2xl bg-background-secondary-default ps-3"
        >
          {isPending ? <RowsSkeleton rows={4} /> : null}
          {isError ? <Row label={`Could not read the statuses from ${look.label}.`} muted /> : null}
          {(statuses ?? []).map((status) => (
            <StatusRow
              key={status.id}
              status={status}
              picked={settings.states[status.id] ?? ""}
              states={live}
              disabled={!canManage}
              onPick={(state) => save({ states: pickStates(settings.states, status.id, state) })}
            />
          ))}
        </div>
      </ModalBody>
      <ModalFooter>
        <span className="me-auto">
          <QuietRefresh label="Refresh statuses" busy={refresh.isPending} onPress={() => refresh.mutate()} />
        </span>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}

export function TeamStatuses({
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
  const client = useQueryClient();
  const installation = settings.installation_id;
  const look = toolLook(settings.provider);
  const words = teamWordsOf(settings.provider);
  const teams = useQuery(issueOptionsQuery(workspace, installation, "teams"));
  const statuses = useQueries({
    queries: (teams.data ?? []).map((team) => issueOptionsQuery(workspace, installation, "states", team.id)),
  });
  const [open, setOpen] = useState<IssueOption | null>(null);
  const refresh = useMutation({
    mutationFn: () => refreshIssueOptions(workspace, installation, "teams"),
    onSuccess: (fresh) => {
      client.setQueryData(issueKeys.options(workspace, installation, "teams", ""), fresh);
      toast.success(`${words.many.charAt(0).toUpperCase()}${words.many.slice(1)} read again from ${look.label}.`);
    },
    onError: (error) => toast.error((error as Error).message),
  });

  return (
    <>
      <Section
        title="Statuses"
        description={`Pick where each ${words.one}'s statuses move linked tickets.`}
        loading={teams.isPending}
        action={
          <QuietRefresh label={`Refresh ${words.many}`} busy={refresh.isPending} onPress={() => refresh.mutate()} />
        }
      >
        {teams.isError ? <Row label={`Could not read the ${words.many} from ${look.label}.`} muted /> : null}
        {teams.data?.length === 0 ? (
          <Row label={`No ${words.many} in ${look.label} yet`} description="Add one there, then refresh." muted />
        ) : null}
        {(teams.data ?? []).map((team, i) => (
          <TeamRow
            key={team.id}
            provider={settings.provider}
            team={team}
            detail={mappedLine(statuses[i]?.data, settings.states, statuses[i]?.isError ?? false)}
            isDefault={team.id === settings.team_id}
            onOpen={() => setOpen(team)}
          />
        ))}
      </Section>
      {open ? (
        <StatusesModal
          workspace={workspace}
          settings={settings}
          team={open}
          canManage={canManage}
          save={save}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </>
  );
}
