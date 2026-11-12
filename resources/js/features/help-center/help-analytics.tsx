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
import { Empty, Glyph, Item, Section } from "@/features/settings/layout";
import { rowLink } from "@/features/settings/row-link";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { type HelpCenterAnalytics, helpCenterAnalyticsQuery } from "@/lib/api/help-center-sites";
import type { HelpCenter } from "@/lib/api/help-centers";

const ranges = [
  ["7", "7 days"],
  ["30", "30 days"],
  ["90", "90 days"],
] as const;

function plural(n: number, one: string, many: string): string {
  return `${n.toLocaleString()} ${n === 1 ? one : many}`;
}

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
    <Panel label="Help center analytics" className="h-full w-full flex-1">
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
      <ScrollArea>
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
          caption: plural(answers, "answer", "answers"),
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
 * Daily draws the views as bars, one a day, tallest the busiest, the part
 * read in the messenger in a lighter shade on top.
 */
function Daily({ data }: { data: HelpCenterAnalytics }) {
  const most = Math.max(1, ...data.daily.map((d) => d.views));

  return (
    <section aria-label="Views by day" className="flex flex-col gap-2">
      <h2 className="px-3 text-body-2-medium text-text-secondary">Views by day</h2>
      <div className="flex h-40 items-end gap-0.5 rounded-2xl bg-background-secondary-default p-4">
        {data.daily.map((day) => (
          <span
            key={day.date}
            title={`${day.date}: ${plural(day.views, "view", "views")}${day.messenger ? `, ${day.messenger} in the messenger` : ""}`}
            className="flex h-full min-w-0 flex-1 flex-col justify-end"
          >
            <span
              className="w-full rounded-t-sm bg-chart-2 transition-[height] duration-500 ease-out"
              style={{ height: day.messenger === 0 ? "0" : `${(day.messenger / most) * 100}%` }}
            />
            <span
              className="w-full bg-chart-1 transition-[height] duration-500 ease-out"
              style={{
                height: day.views === 0 ? "2px" : `${Math.max(4, ((day.views - day.messenger) / most) * 100)}%`,
              }}
            />
          </span>
        ))}
      </div>
      <div className="flex justify-between px-3 text-caption-1-regular text-text-tertiary">
        <span>{data.daily[0]?.date}</span>
        <span>{data.daily.at(-1)?.date}</span>
      </div>
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
                {plural(a.views, "view", "views")}
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
              {plural(g.searches, "search", "searches")}
            </span>
          </Item>
        ))}
      </Section>
    </>
  );
}
