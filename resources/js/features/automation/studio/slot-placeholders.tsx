/*
 * What a studio slot shows until its feature lands: a quiet empty state
 * in the place the feature will fill.
 */
import { RiCloseLine, RiHistoryLine, RiLineChartLine, RiSparkling2Line, RiTestTubeLine } from "@remixicon/react";
import type { ComponentType } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { EmptyState, PageHeader, Panel } from "@/features/shell/page";

import type { PaneSlotProps, SeriesSlotProps, WorkflowSlotProps } from "./slot-props";

function Quiet({
  icon,
  title,
  description,
}: {
  icon: ComponentType<{ className?: string }>;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <EmptyState icon={icon} title={title} description={description} />
    </div>
  );
}

function QuietPane({ onClose, className, title }: PaneSlotProps & { title: string }) {
  return (
    <Panel label="Operator" className={className}>
      <PageHeader title="Operator" level={2}>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close Operator" onClick={onClose} /> : null}
      </PageHeader>
      <EmptyState icon={RiSparkling2Line} title={title} description="Operator's suggestions for it show here." />
    </Panel>
  );
}

export function QuietAnalytics(_: WorkflowSlotProps) {
  return (
    <Quiet icon={RiLineChartLine} title="No numbers yet" description="Counts for each step show here once it runs." />
  );
}

export function QuietFailures(_: WorkflowSlotProps) {
  return null;
}

export function QuietHistory(_: WorkflowSlotProps | SeriesSlotProps) {
  return <Quiet icon={RiHistoryLine} title="No versions yet" description="Each version you publish shows here." />;
}

export function QuietSimulations(_: WorkflowSlotProps | SeriesSlotProps) {
  return <Quiet icon={RiTestTubeLine} title="No simulations yet" description="Simulations you save show here." />;
}

export function QuietWorkflowOperator({ workflow, ...pane }: WorkflowSlotProps & PaneSlotProps) {
  return <QuietPane {...pane} title={`Work on ${workflow.name}`} />;
}

export function QuietSeriesOperator({ series, ...pane }: SeriesSlotProps & PaneSlotProps) {
  return <QuietPane {...pane} title={`Work on ${series.name}`} />;
}
