/*
 * How the team uses Operator, on the agent's page beside dimmi: whether
 * it is on and which model it answers with, what it cost this month and
 * how often it ran, day by day, why it ran, for whom, with which models,
 * what became of the changes it proposed, and its latest runs. Operator
 * and dimmi share one monthly cap, so its spend is shown against both.
 */
import { RiArrowRightSLine, RiChat3Line, RiFileList3Line, RiInboxArchiveLine, RiRobot2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { StatusDot } from "@/components/base/badges/status-dot";
import { Button } from "@/components/base/buttons/button";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { ago } from "@/features/inbox/conversations";
import { Empty, Glyph, Item, Section } from "@/features/settings/layout";
import { rowLink } from "@/features/settings/row-link";
import { avatarColor } from "@/features/shell/avatar-color";
import type { AgentView } from "@/lib/api/ai";
import {
  type OperatorPurpose,
  type OperatorSettings,
  type OperatorUsage,
  operatorSettingsQuery,
  operatorUsageQuery,
} from "@/lib/api/operator";

import { modelsFor, operatorModel, usableKeys, usd } from "../format";
import { ModelLogo } from "../logos";
import { Stat } from "./stat";

const purposes: Record<OperatorPurpose, { label: string; icon: typeof RiChat3Line }> = {
  ask: { label: "Questions from teammates", icon: RiChat3Line },
  brief: { label: "Briefs on conversations", icon: RiFileList3Line },
  triage: { label: "Triage as conversations arrive", icon: RiInboxArchiveLine },
};

/*
 * modelName names a model from the agent's catalog, or by its id.
 */
function modelName(view: AgentView | undefined, model: string): string {
  const entry = Object.values(view?.catalog ?? {})
    .flat()
    .find((candidate) => candidate.id === model);

  return entry?.display_name ?? model;
}

function StatusCard({ settings, view }: { settings: OperatorSettings; view: AgentView | undefined }) {
  const navigate = useNavigate();
  const key = view ? usableKeys(view.keys).find((k) => k.id === view.agent.answer_key_id) : undefined;
  const model = operatorModel(
    settings.model,
    view ? modelsFor(key, view.catalog) : [],
    view?.agent.answer_model ?? null,
  );

  return (
    <div className="flex flex-col gap-4 rounded-2xl bg-background-secondary-default p-4 sm:flex-row sm:items-center">
      <span
        aria-hidden
        className="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-background-primary-default shadow-card"
      >
        <RiRobot2Line className="size-6 text-text-primary" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-title-3-medium text-text-primary">Operator</span>
        <span className="flex items-center gap-2 text-body-2-regular text-text-secondary">
          {settings.enabled ? (
            <StatusDot color="green" />
          ) : (
            <span aria-hidden className="size-2 rounded-full bg-foreground-icon-quaternary" />
          )}
          <span>{settings.enabled ? "Helping your team" : "Switched off"}</span>
          <span aria-hidden>·</span>
          {model ? <ModelLogo model={model} size="xs" /> : null}
          <span className="truncate">{model ? `Answers with ${modelName(view, model)}` : "No model chosen yet"}</span>
        </span>
      </div>
      <Button
        size="small"
        variant="secondary"
        trailingIcon={RiArrowRightSLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "operator" } })}
        className="self-start sm:self-auto"
      >
        Open settings
      </Button>
    </div>
  );
}

/*
 * share says what part of a whole is, as a percentage; a small but real
 * part reads "under 1%" rather than a misleading 0%.
 */
function share(part: number, whole: number): string {
  if (whole <= 0 || part <= 0) {
    return "0%";
  }
  const percent = Math.round((part / whole) * 100);

  return percent < 1 ? "under 1%" : `${percent}%`;
}

function MonthStats({ usage }: { usage: OperatorUsage | undefined }) {
  if (!usage) {
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {["runs", "spend", "changes", "failed"].map((id) => (
          <Skeleton key={id} className="h-[92px] rounded-2xl" />
        ))}
      </div>
    );
  }
  const count = (status: string) => usage.changesets.find((c) => c.status === status)?.count ?? 0;
  const proposed = usage.changesets.reduce((sum, c) => sum + c.count, 0);
  const cap = usage.monthly_spend_cap_micros;

  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
      <Stat label="Runs" value={usage.runs_count.toLocaleString()} />
      <Stat
        label="Spend"
        value={usd(usage.cost_micros)}
        note={
          cap === null
            ? `${share(usage.cost_micros, usage.agent_cost_micros)} of the agent's spend`
            : `${share(usage.cost_micros, cap)} of the ${usd(cap)} cap`
        }
      />
      <Stat label="Changes applied" value={count("applied").toLocaleString()} note={`of ${proposed} proposed`} />
      <Stat label="Failed runs" value={usage.failed_count.toLocaleString()} />
    </div>
  );
}

/*
 * Daily draws the month's runs as bars, one a day, tallest the busiest.
 */
function Daily({ usage }: { usage: OperatorUsage }) {
  const start = new Date(`${usage.period_start}T00:00:00Z`);
  const days = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + 1, 0)).getUTCDate();
  const byDay = new Map(usage.daily.map((d) => [d.day, d]));
  const most = Math.max(1, ...usage.daily.map((d) => d.runs_count));
  const bars = Array.from({ length: days }, (_, index) => {
    const day = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth(), index + 1)).toISOString().slice(0, 10);
    const entry = byDay.get(day);

    return { day, runs: entry?.runs_count ?? 0, cost: entry?.cost_micros ?? 0 };
  });

  return (
    <section aria-label="Runs by day" className="flex flex-col gap-2">
      <h2 className="px-3 text-body-2-medium text-text-secondary">Runs by day</h2>
      <div className="flex h-36 items-end gap-1 rounded-2xl bg-background-secondary-default p-4">
        {bars.map((bar) => (
          <span
            key={bar.day}
            title={`${bar.day}: ${bar.runs} ${bar.runs === 1 ? "run" : "runs"}, ${usd(bar.cost)}`}
            className="flex h-full min-w-0 flex-1 items-end"
          >
            <span
              className="w-full rounded-t-sm bg-chart-1 transition-[height] duration-500 ease-out"
              style={{ height: bar.runs === 0 ? "2px" : `${Math.max(6, (bar.runs / most) * 100)}%` }}
            />
          </span>
        ))}
      </div>
    </section>
  );
}

function Splits({ usage, view }: { usage: OperatorUsage; view: AgentView | undefined }) {
  return (
    <>
      <Section title="Why it ran" description="What started Operator this month.">
        {usage.by_purpose.length === 0 ? <Empty>No runs this month.</Empty> : null}
        {usage.by_purpose.map((row) => (
          <Item
            key={row.purpose}
            mark={<Glyph icon={purposes[row.purpose]?.icon ?? RiChat3Line} />}
            title={purposes[row.purpose]?.label ?? row.purpose}
            detail={row.runs_count === 1 ? "1 run" : `${row.runs_count.toLocaleString()} runs`}
          >
            <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">{usd(row.cost_micros)}</span>
          </Item>
        ))}
      </Section>
      {usage.by_member.length > 0 ? (
        <Section title="By teammate" description="Who Operator worked for.">
          {usage.by_member.map((row) => (
            <Item
              key={row.member_id}
              mark={<Avatar size="md" color={avatarColor(row.name)} initials={row.name.charAt(0).toUpperCase()} />}
              title={row.name}
              detail={row.runs_count === 1 ? "1 run" : `${row.runs_count.toLocaleString()} runs`}
            >
              <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">{usd(row.cost_micros)}</span>
            </Item>
          ))}
        </Section>
      ) : null}
      {usage.by_model.length > 0 ? (
        <Section title="By model" description="What each model cost Operator this month.">
          {usage.by_model.map((row) => (
            <Item
              key={row.model}
              mark={<ModelLogo model={row.model} size="md" tile />}
              title={modelName(view, row.model)}
              detail={row.calls === 1 ? "1 call" : `${row.calls.toLocaleString()} calls`}
            >
              <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">{usd(row.cost_micros)}</span>
            </Item>
          ))}
        </Section>
      ) : null}
    </>
  );
}

function Recent({ usage }: { usage: OperatorUsage }) {
  return (
    <Section title="Recent runs" description="Operator's latest work, newest first.">
      {usage.recent.length === 0 ? <Empty>Runs show here once a teammate asks Operator something.</Empty> : null}
      {usage.recent.map((run) => {
        const purpose = purposes[run.purpose] ?? purposes.ask;
        const title = run.thread_title || purpose.label;
        const who = run.member_name || "Operator";

        return (
          <Item
            key={run.id}
            mark={<Glyph icon={purpose.icon} />}
            title={
              run.conversation_id ? (
                <Link
                  to="/d/inbox/$view/$conversation"
                  params={{ view: "all", conversation: run.conversation_id }}
                  className={rowLink}
                >
                  {title}
                </Link>
              ) : (
                title
              )
            }
            detail={[who, run.conversation_number ? `#${run.conversation_number}` : null, ago(run.created_at)]
              .filter(Boolean)
              .join(" · ")}
          >
            {run.status === "failed" ? (
              <Chip variant="caption" color="rose" className="py-0">
                Failed
              </Chip>
            ) : null}
            <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">{usd(run.cost_micros)}</span>
          </Item>
        );
      })}
    </Section>
  );
}

export function OperatorUsageTab({ workspace, view }: { workspace: string; view: AgentView | undefined }) {
  const { data: settings } = useQuery({ ...operatorSettingsQuery(workspace), enabled: workspace !== "" });
  const { data: usage } = useQuery({ ...operatorUsageQuery(workspace), enabled: workspace !== "" });

  return (
    <>
      {settings ? <StatusCard settings={settings} view={view} /> : <Skeleton className="h-[84px] rounded-2xl" />}
      <section aria-label="Operator this month" className="flex flex-col gap-2">
        <h2 className="px-3 text-body-2-medium text-text-secondary">This month</h2>
        <MonthStats usage={usage} />
      </section>
      {usage ? (
        <>
          <Daily usage={usage} />
          <Splits usage={usage} view={view} />
          <Recent usage={usage} />
        </>
      ) : null}
    </>
  );
}
