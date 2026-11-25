/*
 * Operator's own page, framed like an agent's. Operator works for the team and never answers a
 * customer, so it has none of an agent's drafts, versions or channels:
 * its tabs are how the team used it, what it proposed that waits for
 * review, the routines it runs on its own, its latest runs, and what it
 * may change.
 */
import { RiArrowLeftSLine, RiRobot2Line } from "@remixicon/react";
import { Link } from "@tanstack/react-router";

import { Chip } from "@/components/base/badges/chip";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { OperatorActivity, OperatorOverview, OperatorStatus } from "@/features/ai/overview/operator-usage";
import { useAgent } from "@/features/ai/use-agent";
import { RoutinesBody } from "@/features/routines/routine-list";
import { OperatorSettingsBody } from "@/features/settings/pages/operator";
import { Panel, ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

import { OperatorSuggestions } from "./operator-suggestions";
import { type OperatorTab, operatorTab, operatorTabs } from "./operator-tabs";
import { usePendingCount } from "./suggestions";

const labels: Record<OperatorTab, string> = {
  overview: "Overview",
  suggestions: "Suggestions",
  routines: "Routines",
  activity: "Activity",
  settings: "Settings",
};

/*
 * The column each tab reads in: settings as narrow as the settings
 * pages, the rest as wide as an agent's build tab.
 */
const column = "mx-auto flex w-full animate-page-in flex-col gap-8 px-6 pt-6 pb-16";

export function OperatorPage({ tab, onTab }: { tab: OperatorTab; onTab: (tab: OperatorTab) => void }) {
  const { workspace, view } = useAgent();
  const waiting = usePendingCount(workspace);

  return (
    <Panel label="Operator" ground className="h-full w-full">
      <header className="@container flex h-12 shrink-0 items-center gap-1.5 ps-2.5 pe-2.5 pt-[7px]">
        <Link
          to="/d/agents"
          aria-label="Back to agents"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
        </Link>
        <span className="flex min-w-0 items-center gap-2 px-1.5">
          <span
            aria-hidden
            className="flex size-6 shrink-0 items-center justify-center rounded-md bg-background-tertiary-default"
          >
            <RiRobot2Line className="size-4 text-foreground-icon-secondary" />
          </span>
          <h1 className="truncate text-body-medium text-text-primary">Operator</h1>
        </span>
        <OperatorStatus workspace={workspace} view={view.data} />
      </header>
      <div className="shrink-0 px-4">
        <Tabs selectedKey={tab} onSelectionChange={(key) => onTab(operatorTab(key))}>
          <TabList aria-label="Operator" items={operatorTabs.map((id) => ({ id, label: labels[id] }))}>
            {(item) => (
              <Tab id={item.id}>
                <span className="flex items-center gap-1.5">
                  {item.label}
                  {item.id === "suggestions" && waiting > 0 ? (
                    <Chip variant="caption" color="blue" className="py-0 tabular-nums">
                      {waiting.toLocaleString()}
                    </Chip>
                  ) : null}
                </span>
              </Tab>
            )}
          </TabList>
        </Tabs>
      </div>
      <ScrollArea>
        <div key={tab} className={cx(column, tab === "settings" ? "max-w-[680px]" : "max-w-[760px]")}>
          {tab === "overview" ? <OperatorOverview workspace={workspace} view={view.data} /> : null}
          {tab === "suggestions" ? <OperatorSuggestions workspace={workspace} /> : null}
          {tab === "routines" ? <RoutinesBody /> : null}
          {tab === "activity" ? <OperatorActivity workspace={workspace} /> : null}
          {tab === "settings" ? <OperatorSettingsBody /> : null}
        </div>
      </ScrollArea>
    </Panel>
  );
}
