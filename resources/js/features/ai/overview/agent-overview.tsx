/*
 * The agent at a glance: whether it is answering customers or what
 * still stands in the way, how this month went, and the conversations
 * it took part in lately, each opening in the inbox. Changing anything
 * happens in the agent's settings, one click away.
 */
import { RiArrowRightSLine, RiSettings3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { StatusDot } from "@/components/base/badges/status-dot";
import { Button } from "@/components/base/buttons/button";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { DimmiIcon } from "@/features/brand/logo";
import { AiBadge } from "@/features/inbox/ai-badge";
import { ago, customerName, subjectOf } from "@/features/inbox/conversations";
import { Empty, Item, RowsSkeleton, Section } from "@/features/settings/layout";
import { rowLink } from "@/features/settings/row-link";
import { avatarColor } from "@/features/shell/avatar-color";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { type AgentView, type AiUsage, aiUsageQuery } from "@/lib/api/ai";
import { conversationsQuery } from "@/lib/api/inbox";

import { modelsFor, readiness, usableKeys, usd } from "../format";
import { ModelLogo } from "../logos";
import { useAgent } from "../use-agent";

/*
 * StatusCard says whether the agent answers, and with what.
 */
function StatusCard({ view }: { view: AgentView }) {
  const navigate = useNavigate();
  const { agent } = view;
  const open = readiness(view).filter((step) => !step.done).length;
  const key = usableKeys(view.keys).find((k) => k.id === agent.answer_key_id);
  const model = modelsFor(key, view.catalog).find((m) => m.id === agent.answer_model);
  const state = agent.enabled
    ? { dot: "green" as const, text: "Answering customers" }
    : open > 0
      ? { dot: "yellow" as const, text: `Setup not finished · ${open === 1 ? "1 step" : `${open} steps`} left` }
      : { dot: undefined, text: "Paused" };
  const detail = model ? `Writes with ${model.display_name}` : "No model chosen yet";

  return (
    <div className="flex flex-col gap-4 rounded-2xl bg-background-secondary-default p-4 sm:flex-row sm:items-center">
      <span
        aria-hidden
        className="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-background-primary-default shadow-card"
      >
        <DimmiIcon title="" className="size-6 text-text-primary" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-title-3-medium text-text-primary">{agent.name}</span>
        <span className="flex items-center gap-2 text-body-2-regular text-text-secondary">
          {state.dot ? (
            <StatusDot color={state.dot} />
          ) : (
            <span aria-hidden className="size-2 rounded-full bg-foreground-icon-quaternary" />
          )}
          <span>{state.text}</span>
          <span aria-hidden>·</span>
          {model ? <ModelLogo model={model.id} size="xs" /> : null}
          <span className="truncate">{detail}</span>
        </span>
      </div>
      <Button
        size="small"
        variant={open > 0 ? "primary" : "secondary"}
        trailingIcon={RiArrowRightSLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "agent" } })}
        className="self-start sm:self-auto"
      >
        {open > 0 ? "Finish setup" : "Open settings"}
      </Button>
    </div>
  );
}

function Stat({ label, value, note }: { label: string; value: ReactNode; note?: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-2xl bg-background-secondary-default p-4">
      <span className="text-body-2-regular text-text-secondary">{label}</span>
      <span className="text-title-2-medium text-text-primary tabular-nums">{value}</span>
      {note ? <span className="text-caption-1-medium text-text-tertiary">{note}</span> : null}
    </div>
  );
}

/*
 * MonthStats is this month's answers and spend.
 */
function MonthStats({ usage, cap }: { usage: AiUsage | undefined; cap: number | null }) {
  if (!usage) {
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {["answered", "handed", "unknown", "spend"].map((id) => (
          <Skeleton key={id} className="h-[92px] rounded-2xl" />
        ))}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
      <Stat label="Answered" value={usage.answered_count.toLocaleString()} />
      <Stat label="Handed over" value={usage.handoff_count.toLocaleString()} />
      <Stat label="Didn't know" value={usage.unknown_count.toLocaleString()} />
      <Stat label="Spend" value={usd(usage.cost_micros)} note={cap === null ? "No cap" : `of ${usd(cap)} cap`} />
    </div>
  );
}

/*
 * ByModel is this month's spend split by the model that ran, each named
 * from the catalog when it knows the model.
 */
function ByModel({ usage, view }: { usage: AiUsage | undefined; view: AgentView | undefined }) {
  const rows = usage?.by_model ?? [];
  if (rows.length === 0) {
    return null;
  }
  const names = new Map(
    Object.values(view?.catalog ?? {})
      .flat()
      .map((entry) => [entry.id, entry.display_name]),
  );

  return (
    <Section title="By model" description="What each model cost this month.">
      {rows.map((row) => (
        <Item
          key={row.model}
          mark={<ModelLogo model={row.model} size="md" tile />}
          title={names.get(row.model) ?? row.model}
          detail={row.calls === 1 ? "1 call" : `${row.calls.toLocaleString()} calls`}
        >
          <span className="text-body-2-medium text-text-primary tabular-nums">{usd(row.cost_micros)}</span>
        </Item>
      ))}
    </Section>
  );
}

/*
 * RecentConversations lists the latest conversations the agent took
 * part in.
 */
function RecentConversations({ workspace }: { workspace: string }) {
  const { data: conversations } = useQuery({
    ...conversationsQuery(workspace, { view: "all", status: "any", ai: "any", limit: 8 }),
    enabled: workspace !== "",
  });

  return (
    <Section
      title="Recent conversations"
      description="Conversations the agent answered or handed over."
      action={
        <Link
          to="/d/inbox/$view"
          params={{ view: "all" }}
          search={{ ai: "any", status: "any" }}
          className="rounded-md text-body-2-medium text-text-secondary outline-none hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          View all in inbox
        </Link>
      }
    >
      {conversations === undefined ? <RowsSkeleton rows={3} /> : null}
      {conversations?.length === 0 ? <Empty>Nothing yet. Conversations show here once the agent answers.</Empty> : null}
      {conversations?.map((conversation) => {
        const name = customerName(conversation.contact);

        return (
          <Item
            key={conversation.id}
            mark={<Avatar size="md" color={avatarColor(name)} initials={name.charAt(0).toUpperCase()} />}
            title={
              <Link
                to="/d/inbox/$view/$conversation"
                params={{ view: "all", conversation: conversation.id }}
                className={rowLink}
              >
                {name}
              </Link>
            }
            detail={subjectOf(conversation)}
          >
            <AiBadge state={conversation.ai_state} />
            <span className="hidden text-caption-1-medium text-text-tertiary sm:inline">
              {ago(conversation.last_message_at ?? conversation.created_at)}
            </span>
          </Item>
        );
      })}
    </Section>
  );
}

export function AgentOverview() {
  const navigate = useNavigate();
  const { workspace, view } = useAgent();
  const { data: usage } = useQuery({ ...aiUsageQuery(workspace), enabled: workspace !== "" });

  return (
    <Panel label="Agent" ground className="h-full flex-1">
      <PageHeader title="Agent">
        <Button
          size="small"
          variant="secondary"
          leadingIcon={RiSettings3Line}
          onClick={() => navigate({ to: "/d/settings/$section", params: { section: "agent" } })}
        >
          Settings
        </Button>
      </PageHeader>
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-4 pt-4 pb-16 sm:px-6 sm:pt-8">
          {view.data ? <StatusCard view={view.data} /> : <Skeleton className="h-[84px] rounded-2xl" />}
          <section aria-label="This month" className="flex flex-col gap-2">
            <h2 className="px-3 text-body-2-medium text-text-secondary">This month</h2>
            <MonthStats usage={usage} cap={view.data?.agent.monthly_spend_cap_micros ?? null} />
          </section>
          <ByModel usage={usage} view={view.data} />
          <RecentConversations workspace={workspace} />
        </div>
      </ScrollArea>
    </Panel>
  );
}
