/*
 * Settings → SLA policies: the workspace's service level policies, each
 * with its targets, the hours they count in, what it applies to and what
 * a breach changes. A row opens the policy's editor; a member who may
 * change settings adds one from the page's action. Archiving asks first,
 * since it cancels the clocks the policy runs. Archived policies sit at
 * the end, marked.
 */
import { RiArchiveLine, RiEditLine, RiTimerLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { TFunction } from "i18next";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Chip } from "@/components/base/badges/chip";
import { Action } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Empty, Glyph, Item, Section, SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { RowMenu } from "@/features/shell/row-menu";
import { schedulesQuery } from "@/lib/api/schedules";
import type { SLAPolicy } from "@/lib/api/sla";
import { slaPoliciesQuery } from "@/lib/api/sla";
import { teamsQuery } from "@/lib/api/teams";
import type { TicketPriority } from "@/lib/api/tickets";
import { ticketTypesQuery } from "@/lib/api/tickets";
import { activeLocale } from "@/lib/i18n";

import { priorityLabel } from "../labels";
import { ArchivePolicyDialog } from "./archive-dialog";
import { formatDuration, ordered, targetKeys, targetsOf } from "./form";

type Names = Map<string, string>;

function targetsLine(t: TFunction<"sla">, policy: SLAPolicy): string {
  const targets = targetsOf(policy);
  const parts = targetKeys
    .filter((key) => targets[key] !== null)
    .map((key) => `${t(`targets.${key}`)} ${formatDuration(targets[key] ?? 0, activeLocale())}`);

  return parts.length > 0 ? parts.join(" · ") : t("list.noTargets");
}

function scopeLine(t: TFunction<"sla">, policy: SLAPolicy, names: { schedules: Names; types: Names; teams: Names }) {
  const hours = policy.schedule_id ? (names.schedules.get(policy.schedule_id) ?? "") : t("list.allHours");
  const applies = [
    ...policy.ticket_type_ids.map((id) => names.types.get(id) ?? ""),
    ...policy.priorities.map((p) => priorityLabel(p as TicketPriority)),
  ].filter(Boolean);
  const breach = [
    policy.breach_priority ? priorityLabel(policy.breach_priority as TicketPriority) : "",
    policy.breach_team_id ? (names.teams.get(policy.breach_team_id) ?? "") : "",
  ].filter(Boolean);

  return [
    hours,
    applies.length > 0 ? t("list.appliesTo", { what: applies.join(", ") }) : t("list.appliesToNothing"),
    breach.length > 0 ? t("list.breach", { what: breach.join(", ") }) : "",
  ]
    .filter(Boolean)
    .join(" · ");
}

function byId(list: { id: string; name: string }[] | undefined): Names {
  return new Map((list ?? []).map((x) => [x.id, x.name]));
}

export function SLASettings() {
  const { t } = useTranslation("sla");
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const canChange = usePermission(workspace, "settings:write");
  const { data: policies = [], isPending } = useQuery(slaPoliciesQuery(workspace));
  const schedules = useQuery({ ...schedulesQuery(workspace), enabled: workspace !== "" }).data;
  const types = useQuery(ticketTypesQuery(workspace)).data;
  const teams = useQuery({ ...teamsQuery(workspace), enabled: workspace !== "" }).data;
  const names = { schedules: byId(schedules), types: byId(types), teams: byId(teams) };
  const [archiving, setArchiving] = useState<SLAPolicy | null>(null);
  const open = (policy: string) => void navigate({ to: "/d/settings/sla/$policy", params: { policy } });
  const actions = [
    { kind: "edit" as const, label: t("list.edit"), icon: RiEditLine },
    { kind: "archive" as const, label: t("list.archive"), icon: RiArchiveLine, danger: true },
  ];

  return (
    <SettingsPage
      title={t("page.title")}
      description={t("page.description")}
      actions={canChange ? <Action onPress={() => open("new")}>{t("list.new")}</Action> : null}
    >
      <Section title={t("list.title")} description={t("list.description")} loading={isPending}>
        {policies.length === 0 ? <Empty>{t("list.empty")}</Empty> : null}
        {ordered(policies).map((policy) => (
          <Item
            key={policy.id}
            mark={<Glyph icon={RiTimerLine} />}
            title={
              <button
                type="button"
                onClick={() => open(policy.id)}
                className="text-start outline-none after:absolute after:inset-0 focus-visible:underline"
              >
                {policy.name}
              </button>
            }
            detail={
              <>
                <span className="block truncate">{targetsLine(t, policy)}</span>
                <span className="block truncate text-text-tertiary">{scopeLine(t, policy, names)}</span>
              </>
            }
          >
            {policy.archived_at ? (
              <Chip variant="caption" color="neutral">
                {t("list.archived")}
              </Chip>
            ) : null}
            {canChange && !policy.archived_at ? (
              <RowMenu
                label={policy.name}
                actions={actions}
                onPick={(kind) => (kind === "edit" ? open(policy.id) : setArchiving(policy))}
              />
            ) : null}
          </Item>
        ))}
      </Section>
      {archiving ? (
        <ArchivePolicyDialog workspace={workspace} policy={archiving} onClose={() => setArchiving(null)} />
      ) : null}
    </SettingsPage>
  );
}
