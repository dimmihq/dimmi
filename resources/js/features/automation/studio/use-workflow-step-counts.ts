/*
 * useWorkflowStepCounts reads a workflow's per-step numbers from its
 * analytics, the last 30 days by default, and maps each step id to what
 * it did: runs that reached it, wait on it now, failed on it, ended on
 * it without completing, and left by each path.
 */
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { type WorkflowStepMetrics, workflowMetricsQuery } from "@/lib/api/workflows";

import { daysBack } from "./workflow-numbers";

export type StepCountsMap = ReadonlyMap<string, WorkflowStepMetrics>;

const none: StepCountsMap = new Map();

export function useWorkflowStepCounts(
  workspace: string,
  workflow: string,
  { days = 30, version = 0 }: { days?: number; version?: number } = {},
): { counts: StepCountsMap; loading: boolean } {
  const query = useQuery(workflowMetricsQuery(workspace, workflow, { from: daysBack(days), version }));
  const counts = useMemo(() => (query.data ? new Map(Object.entries(query.data.steps)) : none), [query.data]);

  return { counts, loading: query.isPending };
}
