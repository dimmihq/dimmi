/*
 * The parts of the workflow and series studios that come from their own
 * features: analytics, failed runs, history, simulations and Operator.
 * Each slot is a component the studio mounts as is. Until a feature
 * lands, its slot shows a quiet empty state; mounting the real thing is
 * one line, such as `analytics: WorkflowAnalytics,` with its import.
 *
 * Every workflow slot gets WorkflowSlotProps, every series slot
 * SeriesSlotProps; Operator slots fill a pane, so they also get
 * PaneSlotProps (onClose, className).
 */
import type { ComponentType } from "react";
import { SeriesOperatorPanel } from "@/features/series/studio/series-operator-panel";
import { QuietHistory, QuietSimulations } from "./slot-placeholders";

import type { PaneSlotProps, SeriesSlotProps, WorkflowSlotProps } from "./slot-props";
import { WorkflowAnalytics } from "./workflow-analytics";
import { WorkflowFailures } from "./workflow-failures";
import { WorkflowHistory } from "./workflow-history";
import { WorkflowOperatorPanel } from "./workflow-operator-panel";

export type { PaneSlotProps, SeriesSlotProps, WorkflowSlotProps } from "./slot-props";

/*
 * The workflow studio's slots. On the Analytics tab, analytics sits on
 * top, failed runs below it, then the recent runs the studio lists
 * itself. History fills its tab, Simulations the side panel's tab, and
 * Operator its own pane.
 */
export const workflowSlots: {
  analytics: ComponentType<WorkflowSlotProps>;
  failures: ComponentType<WorkflowSlotProps>;
  history: ComponentType<WorkflowSlotProps>;
  simulations: ComponentType<WorkflowSlotProps>;
  operator: ComponentType<WorkflowSlotProps & PaneSlotProps>;
} = {
  analytics: ({ workspace, workflow }) => <WorkflowAnalytics workspace={workspace} workflow={workflow.id} />,
  failures: ({ workspace, workflow }) => <WorkflowFailures workspace={workspace} workflow={workflow.id} />,
  history: ({ workspace, workflow, onAdopt }) => (
    <WorkflowHistory workspace={workspace} workflow={workflow.id} onRestored={onAdopt} />
  ),
  simulations: QuietSimulations,
  operator: ({ workspace, workflow, onClose, className }) => (
    <WorkflowOperatorPanel workspace={workspace} workflow={workflow.id} onClose={onClose} className={className} />
  ),
};

/*
 * The series studio's slots. Its Analytics tab is the canvas read with
 * each step's results, which the series builder draws itself.
 */
export const seriesSlots: {
  history: ComponentType<SeriesSlotProps>;
  simulations: ComponentType<SeriesSlotProps>;
  operator: ComponentType<SeriesSlotProps & PaneSlotProps>;
} = {
  history: QuietHistory,
  simulations: QuietSimulations,
  operator: ({ workspace, series, onClose, className }) => (
    <SeriesOperatorPanel workspace={workspace} series={series.id} onClose={onClose} className={className} />
  ),
};
