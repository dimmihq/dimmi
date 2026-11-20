/*
 * What readers did with the changelog over the last days: how many
 * times updates were opened, day by day; the most read updates with the
 * reactions readers left; and how the list of subscribers moved. Only
 * counts are kept, never who read.
 */
import { RiMegaphoneLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { CartesianChart } from "@/features/reports/charts";
import { Empty, Glyph, Item, Section } from "@/features/settings/layout";
import { rowLink } from "@/features/settings/row-link";
import { changelogAnalyticsQuery, type ChangelogAnalytics as Report } from "@/lib/api/changelog";
import type { ReportChart } from "@/lib/api/reports";
import { countOf, formatNumber } from "@/lib/i18n/format";

const ranges = [
  ["7", "7 days"],
  ["30", "30 days"],
  ["90", "90 days"],
] as const;

/*
 * DaysPicker picks how far back the analytics reach.
 */
export function DaysPicker({ days, onDays }: { days: number; onDays: (days: number) => void }) {
  return (
    <SegmentedControl
      aria-label="Days"
      selectedKeys={new Set([String(days)])}
      onSelectionChange={(keys) => {
        const [key] = [...keys];
        if (key !== undefined) {
          onDays(Number(key));
        }
      }}
    >
      {ranges.map(([id, name]) => (
        <SegmentedControlItem key={id} id={id}>
          {name}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

function Totals({ data }: { data: Report }) {
  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        {
          label: "Views",
          hint: "Times readers opened an update's page on the changelog. Crawlers are left out.",
          value: formatNumber(data.views),
        },
        {
          label: "Reactions",
          hint: "Reactions readers left on updates over these days.",
          value: formatNumber(data.reactions),
        },
        {
          label: "Subscribers",
          hint: "People who confirmed they want new updates by email.",
          value: formatNumber(data.subscribers),
          caption: `${formatNumber(data.subscribers_joined)} joined`,
        },
        {
          label: "Unsubscribed",
          hint: "Subscribers who left over these days.",
          value: formatNumber(data.subscribers_unsubscribed),
        },
      ]}
    />
  );
}

function Daily({ data }: { data: Report }) {
  const chart: ReportChart = {
    key: "views_by_day",
    title: "Views by day",
    kind: "bar",
    unit: "count",
    axis: "day",
    drill: null,
    columns: [],
    rows: [],
    series: [{ key: "views", label: "Views", drill: null, points: data.daily.map((d) => ({ x: d.date, y: d.views })) }],
  };

  return (
    <section
      aria-label={chart.title}
      className="flex min-w-0 flex-col gap-3 rounded-2xl bg-background-secondary-default p-4"
    >
      <h2 className="truncate text-body-medium text-text-primary">{chart.title}</h2>
      <CartesianChart chart={chart} onPick={() => undefined} />
    </section>
  );
}

export function ChangelogAnalytics({
  workspace,
  helpCenter,
  days,
}: {
  workspace: string;
  helpCenter: string;
  days: number;
}) {
  const { data } = useQuery({ ...changelogAnalyticsQuery(workspace, helpCenter, days), enabled: helpCenter !== "" });
  if (!data) {
    return (
      <div className="flex flex-col gap-6">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {["views", "reactions", "subscribers", "left"].map((id) => (
            <Skeleton key={id} className="h-[100px] rounded-2xl" />
          ))}
        </div>
        <Skeleton className="h-72 rounded-2xl" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 pb-4">
      <Totals data={data} />
      <Daily data={data} />
      <Section title="Most read" description="The updates readers opened most, with the reactions they left.">
        {data.top_entries.length === 0 ? <Empty>No views yet.</Empty> : null}
        {data.top_entries.map((e) => (
          <Item
            key={e.id}
            mark={<Glyph icon={RiMegaphoneLine} />}
            title={
              <Link to="/d/changelog/$entry" params={{ entry: e.id }} className={rowLink}>
                {e.title}
              </Link>
            }
            detail={countOf(e.reactions, "reaction", "reactions")}
          >
            <span className="pe-3 text-body-2-medium text-text-primary tabular-nums">
              {countOf(e.views, "view", "views")}
            </span>
          </Item>
        ))}
      </Section>
    </div>
  );
}
