/*
 * Home is where a teammate starts the day: who needs a person now, what
 * Operator found, how the week is going and how the agents are doing.
 * Each line opens the page that owns it; Home keeps nothing of its own
 * and reads what the inbox, Operator and the reports already serve. A
 * block with nothing in it says what will show there and how to fill it.
 */
import {
  RiArrowRightSLine,
  RiAtLine,
  RiCheckboxBlankCircleLine,
  RiCheckboxCircleFill,
  RiHome5Line,
  RiInbox2Line,
  RiQuestionAnswerLine,
  RiStackLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { ComponentType } from "react";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { customerName, subjectOf, useInbox } from "@/features/inbox/conversations";
import { ButtonLink } from "@/features/navigation/links";
import { formatValue } from "@/features/reports/format";
import { presetDays } from "@/features/reports/search";
import { Section } from "@/features/settings/layout";
import { avatarColor } from "@/features/shell/avatar-color";
import { Panel, ScrollArea } from "@/features/shell/page";
import { useSteps } from "@/features/shell/setup-steps";
import { type Counts, conversationsQuery, countsQuery } from "@/lib/api/inbox";
import { type Report, reportQuery } from "@/lib/api/reports";
import { callerQuery } from "@/lib/api/workspace";
import { formatDate } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { changeOf, initialsOf, metricOf, waited } from "./figures";
import { HomeOperator } from "./home-operator";
import { Blank, Row, Rows } from "./rows";

const tile =
  "flex flex-col gap-1 rounded-2xl bg-background-secondary-default p-4 outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring";

const arrow = <RiArrowRightSLine aria-hidden className="size-5 text-foreground-icon-tertiary rtl:-scale-x-100" />;

/* How many waiting customers Home names. */
const named = 5;

interface Tile {
  view: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
  value: (counts: Counts) => string;
}

const tiles: Tile[] = [
  { view: "mine", label: "Yours", icon: RiInbox2Line, value: (c) => c.views.mine.open.toLocaleString() },
  {
    view: "unassigned",
    label: "Unassigned",
    icon: RiQuestionAnswerLine,
    value: (c) => c.views.unassigned.open.toLocaleString(),
  },
  {
    view: "all",
    label: "All open",
    icon: RiStackLine,
    value: (c) => `${c.views.all.open.toLocaleString()}${c.views.all.capped ? "+" : ""}`,
  },
  {
    view: "mentions",
    label: "Unread mentions",
    icon: RiAtLine,
    value: (c) => c.views.mentions.unread.toLocaleString(),
  },
];

/*
 * Queue is who needs a person now: the open conversations by whose they
 * are, each count opening the inbox on them, then the customers who have
 * waited longest for the team.
 */
function Queue({ workspace }: { workspace: string }) {
  const on = { enabled: workspace !== "" } as const;
  const { data: counts } = useQuery({ ...countsQuery(workspace), ...on });
  const { data: longest } = useQuery({
    ...conversationsQuery(workspace, { view: "all", status: "open", sort: "waiting_since", limit: named }),
    ...on,
  });
  const waiting = (longest ?? []).filter((conversation) => conversation.waiting_since);

  return (
    <section aria-label="Needs a person now" className="flex flex-col gap-2">
      <h2 className="px-3 text-body-2-medium text-text-secondary">Needs a person now</h2>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {tiles.map((t) => (
          <Link key={t.view} to="/d/inbox/$view" params={{ view: t.view }} className={tile}>
            <span className="flex items-center gap-1.5 text-body-2-regular text-text-secondary">
              <t.icon aria-hidden className="size-4 text-foreground-icon-secondary" />
              {t.label}
            </span>
            {counts ? (
              <span className="text-title-2-medium text-text-primary tabular-nums">{t.value(counts)}</span>
            ) : (
              <Skeleton shape="text" className="my-1 h-5 w-8" />
            )}
          </Link>
        ))}
      </div>
      {longest === undefined ? (
        <Skeleton className="h-[52px] rounded-2xl" />
      ) : (
        <Rows>
          {waiting.length === 0 ? (
            <Blank
              action={
                <ButtonLink variant="tertiary" size="medium" to="/d/settings">
                  Connect a channel
                </ButtonLink>
              }
            >
              Nobody is waiting. Customers who need a person show here, longest wait first.
            </Blank>
          ) : null}
          {waiting.map((conversation) => {
            const name = customerName(conversation.contact);
            const wait = waited(conversation.waiting_since ?? "");

            return (
              <Row
                key={conversation.id}
                where={{ to: "/d/inbox/$view/$conversation", params: { view: "all", conversation: conversation.id } }}
                mark={
                  <Avatar
                    size="sm"
                    color={avatarColor(name)}
                    src={conversation.contact.avatar_url ?? undefined}
                    initials={initialsOf(name)}
                  />
                }
                title={`${name} · ${subjectOf(conversation)}`}
                detail={`#${conversation.number} · ${conversation.inbox.name} · waiting ${wait}`}
              >
                {conversation.priority === "urgent" ? (
                  <Chip variant="caption" color="rose" className="py-0">
                    Urgent
                  </Chip>
                ) : null}
                {arrow}
              </Row>
            );
          })}
        </Rows>
      )}
    </section>
  );
}

/*
 * Setup is a workspace's first steps, each with why it matters and
 * checked off once done. It leaves Home when every step is done, and
 * never shows to a person who may not change settings.
 */
function Setup({ workspace, fresh }: { workspace: string; fresh: boolean }) {
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const steps = useSteps(workspace, caller?.member?.permissions?.includes("settings:read") ?? false);
  const done = (steps ?? []).filter((step) => step.done).length;
  if (!steps || done === steps.length) {
    return null;
  }

  return (
    <>
      {fresh ? (
        <div className="flex flex-col gap-1 px-3">
          <h2 className="text-title-2-medium text-text-primary">Welcome to dimmi.</h2>
          <p className="text-body-regular text-text-secondary">
            This page fills up as customers write in. Until then, here is what to set up first.
          </p>
        </div>
      ) : null}
      <Section title="Get set up" description={`${done} of ${steps.length} done. Each step takes a few minutes.`} bare>
        <Rows>
          {steps.map((step) => {
            const Mark = step.done ? RiCheckboxCircleFill : RiCheckboxBlankCircleLine;

            return (
              <Row
                key={step.label}
                where={{ to: step.href }}
                muted={step.done}
                mark={
                  <Mark
                    aria-hidden
                    className={cx(
                      "size-5",
                      step.done ? "text-foreground-icon-secondary" : "text-foreground-icon-tertiary",
                    )}
                  />
                }
                title={step.label}
                detail={step.why}
              >
                {step.done ? null : (
                  <RiArrowRightSLine aria-hidden className="size-5 text-foreground-icon-tertiary rtl:-scale-x-100" />
                )}
              </Row>
            );
          })}
        </Rows>
      </Section>
    </>
  );
}

/*
 * Week is the last seven days in four numbers, each against the seven
 * before: how many wrote in, how much the AI resolved, how fast the
 * first answer came and what customers thought.
 */
function Week({ overview, ai }: { overview: Report | undefined; ai: Report | undefined }) {
  if (!overview || !ai) {
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {["conversations", "ai", "reply", "rating"].map((id) => (
          <Skeleton key={id} className="h-[100px] rounded-2xl" />
        ))}
      </div>
    );
  }
  if (!metricOf(overview, "new_conversations")?.value) {
    return (
      <Rows>
        <Blank>Conversations, reply time and ratings show here after your first conversations.</Blank>
      </Rows>
    );
  }
  const stat = (key: string, from: Report, label: string, hint: string) => {
    const metric = metricOf(from, key);

    return { label, hint, value: formatValue(metric?.unit ?? "count", metric?.value), caption: changeOf(metric) };
  };

  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        stat("new_conversations", overview, "Conversations", "Conversations customers started in the last 7 days."),
        stat("ai_resolution", ai, "Resolved by AI", "Share of the conversations an AI agent took that it resolved."),
        stat("first_response", overview, "First reply", "Median time a customer waited for the first answer."),
        stat("csat", overview, "Rating", "Share of rated conversations the customer rated good."),
      ]}
    />
  );
}

/*
 * Agents is one line for each AI agent that took conversations this
 * week: how many, how much it resolved and how much it passed on.
 */
function Agents({ ai }: { ai: Report | undefined }) {
  if (!ai) {
    return <Section title="Agents" loading rows={2} />;
  }
  const table = ai.charts.find((chart) => chart.key === "ai_agents");
  const rows = table?.rows ?? [];
  const at = (key: string) => table?.columns.findIndex((column) => column.key === key) ?? -1;
  const [involved, resolution, handoff] = [at("involved"), at("resolution"), at("handoff")];

  return (
    <Section title="Agents" description="How each AI agent did in the last 7 days." bare>
      <Rows>
        {rows.length === 0 ? (
          <Blank
            action={
              <ButtonLink variant="tertiary" size="medium" to="/d/agents">
                Set up an agent
              </ButtonLink>
            }
          >
            No agent answered customers this week. One can take the common questions from your docs.
          </Blank>
        ) : null}
        {rows.map((row) => {
          const taken = formatValue("count", row.values[involved]);
          const passed = formatValue("percent", row.values[handoff]);

          return (
            <Row
              key={row.key}
              where={
                row.filter.agent ? { to: "/d/agents/$agent", params: { agent: row.filter.agent } } : { to: "/d/agents" }
              }
              mark={<Avatar size="sm" color={avatarColor(row.label)} initials={initialsOf(row.label)} />}
              title={row.label}
              detail={`${taken} conversations · handed over ${passed}`}
            >
              <span className="text-body-2-medium text-text-primary tabular-nums">
                {formatValue("percent", row.values[resolution])} resolved
              </span>
              {arrow}
            </Row>
          );
        })}
      </Rows>
    </Section>
  );
}

/*
 * Numbers is the part of Home that reads the reports: the week and the
 * agents. A teammate who may not read reports is shown neither.
 */
function Numbers({ workspace }: { workspace: string }) {
  const range = { ...presetDays("7d"), compare: true };
  const on = { enabled: workspace !== "", retry: false } as const;
  const overview = useQuery({ ...reportQuery(workspace, "overview", range), ...on });
  const ai = useQuery({ ...reportQuery(workspace, "ai", range), ...on });
  if (overview.isError || ai.isError) {
    return null;
  }

  return (
    <>
      <section aria-label="This week" className="flex flex-col gap-2">
        <h2 className="px-3 text-body-2-medium text-text-secondary">This week</h2>
        <Week overview={overview.data} ai={ai.data} />
      </section>
      <Agents ai={ai.data} />
    </>
  );
}

export function HomePage() {
  const { workspace } = useInbox();
  const { data: counts } = useQuery({ ...countsQuery(workspace), enabled: workspace !== "" });

  return (
    <Panel label="Home" ground className="h-full w-full">
      <header className="flex h-12 shrink-0 items-center gap-2 ps-4 pe-2.5 pt-[7px]">
        <span
          aria-hidden
          className="flex size-6 shrink-0 items-center justify-center rounded-md bg-background-tertiary-default"
        >
          <RiHome5Line className="size-4 text-foreground-icon-secondary" />
        </span>
        <h1 className="truncate text-body-medium text-text-primary">Home</h1>
        <span className="text-body-2-regular text-text-tertiary">{formatDate(new Date().toISOString())}</span>
      </header>
      <ScrollArea>
        <div className="mx-auto flex w-full max-w-[760px] animate-page-in flex-col gap-8 px-6 pt-6 pb-16">
          <Setup workspace={workspace} fresh={counts?.views.all.open === 0} />
          <Queue workspace={workspace} />
          <HomeOperator workspace={workspace} />
          <Numbers workspace={workspace} />
        </div>
      </ScrollArea>
    </Panel>
  );
}
