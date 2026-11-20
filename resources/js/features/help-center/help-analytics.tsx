/*
 * What readers did with a help center over the last days: how many times
 * articles were opened, day by day, on the help center and in the
 * messenger; the most read articles with how helpful readers found them;
 * and the searches that found nothing, which are the articles still to
 * write. Only counts are kept, never who read.
 */
import { RiArticleLine, RiSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { CartesianChart } from "@/features/reports/charts";
import { seriesColor } from "@/features/reports/format";
import { Empty, Glyph, Item, Section } from "@/features/settings/layout";
import { rowLink } from "@/features/settings/row-link";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { type HelpCenterAnalytics, helpCenterAnalyticsQuery } from "@/lib/api/help-center-sites";
import type { HelpCenter } from "@/lib/api/help-centers";
import type { ReportChart } from "@/lib/api/reports";
import { countOf } from "@/lib/i18n/format";

const ranges = [
  ["7", "7 days"],
  ["30", "30 days"],
  ["90", "90 days"],
] as const;

export function HelpAnalytics({
  workspace,
  center,
  header,
}: {
  workspace: string;
  center: HelpCenter | undefined;
  header?: ReactNode;
}) {
  const [days, setDays] = useState(30);
  const hc = center?.id ?? "";
  const { data } = useQuery({ ...helpCenterAnalyticsQuery(workspace, hc, days), enabled: hc !== "" });

  return (
    <Panel label="Help center analytics" ground className="h-full w-full flex-1">
      <PageHeader title="Analytics">
        {header}
        <SegmentedControl
          aria-label="Days"
          selectedKeys={new Set([String(days)])}
          onSelectionChange={(keys) => {
            const [key] = [...keys];
            if (key !== undefined) {
              setDays(Number(key));
            }
          }}
        >
          {ranges.map(([id, name]) => (
            <SegmentedControlItem key={id} id={id}>
              {name}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </PageHeader>
      <ScrollArea ground>
        <div className="flex flex-col gap-6 px-4 pb-8">
          <Totals data={data} />
          {data ? <Daily data={data} /> : <Skeleton className="h-44 rounded-2xl" />}
          {data ? <Lists data={data} /> : null}
        </div>
      </ScrollArea>
    </Panel>
  );
}

function Totals({ data }: { data: HelpCenterAnalytics | undefined }) {
  if (!data) {
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {["views", "helpful", "searches", "gaps"].map((id) => (
          <Skeleton key={id} className="h-[100px] rounded-2xl" />
        ))}
      </div>
    );
  }
  const messenger = data.daily.reduce((sum, d) => sum + d.messenger, 0);
  const answers = data.helpful + data.unhelpful;

  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        {
          label: "Article views",
          hint: "Times readers opened an article, on the help center and in the messenger. Crawlers are left out.",
          value: data.views.toLocaleString(),
          caption: `${messenger.toLocaleString()} in the messenger`,
        },
        {
          label: "Helpful",
          hint: "Of the readers who answered “Was this article helpful?”, how many said yes.",
          value: data.helpful_percent === null ? "—" : `${data.helpful_percent}%`,
          caption: countOf(answers, "answer", "answers"),
        },
        {
          label: "Searches",
          hint: "Searches readers ran on the help center.",
          value: data.searches.toLocaleString(),
        },
        {
          label: "No results",
          hint: "Searches that found no article. The list below shows what readers looked for.",
          value: data.empty_searches.toLocaleString(),
        },
      ]}
    />
  );
}

/*
 * Daily draws the views as bars, one a day, the part read in the
 * messenger stacked on the part read on the help center. Hovering a day
 * shows its date and both counts.
 */
function Daily({ data }: { data: HelpCenterAnalytics }) {
  const chart: ReportChart = {
    key: "views_by_day",
    title: "Views by day",
    kind: "stacked_bar",
    unit: "count",
    axis: "day",
    drill: null,
    columns: [],
    rows: [],
    series: [
      {
        key: "web",
        label: "Help center",
        drill: null,
        points: data.daily.map((d) => ({ x: d.date, y: d.views - d.messenger })),
      },
      {
        key: "messenger",
        label: "Messenger",
        drill: null,
        points: data.daily.map((d) => ({ x: d.date, y: d.messenger })),
      },
    ],
  };

  return (
    <section
      aria-label={chart.title}
      className="flex min-w-0 flex-col gap-3 rounded-2xl bg-background-secondary-default p-4"
    >
      <header className="flex min-w-0 flex-col gap-1.5">
        <h2 className="truncate text-body-medium text-text-primary">{chart.title}</h2>
        <ul className="flex flex-wrap items-center gap-x-3 gap-y-1">
          {chart.series.map((series, i) => (
            <li key={series.key} className="flex items-center gap-1.5 text-caption-1-regular text-text-secondary">
              <span aria-hidden className="size-2 rounded-full" style={{ background: seriesColor(i) }} />
              {series.label}
            </li>
          ))}
        </ul>
      </header>
      <CartesianChart chart={chart} onPick={() => undefined} />
    </section>
  );
}

function Lists({ data }: { data: HelpCenterAnalytics }) {
  return (
    <>
      <Section title="Most read" description="The articles readers opened most, with how many found them helpful.">
        {data.top_articles.length === 0 ? <Empty>No views yet.</Empty> : null}
        {data.top_articles.map((a) => {
          const answers = a.helpful + a.unhelpful;
          const helpful = answers === 0 ? "No answers yet" : `${Math.round((a.helpful / answers) * 100)}% helpful`;

          return (
            <Item
              key={a.id}
              mark={<Glyph icon={RiArticleLine} />}
              title={
                <Link to="/d/help-center/$article" params={{ article: a.id }} className={rowLink}>
                  {a.title}
                </Link>
              }
              detail={helpful}
            >
              <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">
                {countOf(a.views, "view", "views")}
              </span>
            </Item>
          );
        })}
      </Section>
      <Section
        title="Searches with no results"
        description="What readers looked for and did not find: articles to write."
      >
        {data.content_gaps.length === 0 ? <Empty>Every search found something.</Empty> : null}
        {data.content_gaps.map((g) => (
          <Item key={g.query} mark={<Glyph icon={RiSearchLine} />} title={g.query} detail={`Last on ${g.last_date}`}>
            <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">
              {countOf(g.searches, "search", "searches")}
            </span>
          </Item>
        ))}
      </Section>
    </>
  );
}
