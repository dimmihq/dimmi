import type { Series } from "@/lib/api/series";
import type { Workflow } from "@/lib/api/workflows";

/*
 * What every workflow slot gets: the workspace, and the workflow as the
 * server last sent it with the working copy's name and spec.
 */
export interface WorkflowSlotProps {
  workspace: string;
  workflow: Workflow;
  /* Hands a working copy the slot changed, such as a restored version, to the open editor. */
  onAdopt?: (next: Workflow) => void;
  /* Saves the editor's unsaved changes first, so a run on the draft runs what is on screen. */
  flush?: () => Promise<void>;
}

/*
 * What every series slot gets: the workspace, and the series with the
 * working copy's spec.
 */
export interface SeriesSlotProps {
  workspace: string;
  series: Series;
  /* Hands a working copy the slot changed, such as a restored version, to the open editor. */
  onAdopt?: (next: Series) => void;
  /* The step selected on the canvas, which Operator is told about. */
  step?: string;
}

/*
 * What a slot that fills a pane of its own also gets. A component that
 * ignores them still fits the slot.
 */
export interface PaneSlotProps {
  onClose?: () => void;
  className?: string;
}
