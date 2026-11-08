/*
 * The workspace's procedures: how the team wants dimmi to handle one kind
 * of request. The working copy is saved as it is edited, naming the lock
 * version it started from; dimmi follows only the published copy.
 * Reading takes ai_actions:read; changing takes ai_agent:manage, which
 * the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type ProcedureStepKind = "instruction" | "condition" | "action";

export interface ProcedureStep {
  id: string;
  kind: ProcedureStepKind;
  text: string;
  action_id?: string;
}

/*
 * ProcedureDefinition is a procedure as dimmi reads it: when it applies,
 * the steps in order, and when to hand over.
 */
export interface ProcedureDefinition {
  description: string;
  steps: ProcedureStep[];
  handover: string[];
}

export interface Procedure {
  id: string;
  name: string;
  status: "draft" | "live";
  lock_version: number;
  definition: ProcedureDefinition;
  has_unpublished_changes: boolean;
  uses: number;
  published_at: string | null;
  updated_at: string;
}

/*
 * ProcedureIssue is one problem that stops a procedure from being
 * published.
 */
export interface ProcedureIssue {
  code: string;
  step_id?: string;
  field?: string;
  message: string;
}

const path = (id: string) => `/procedures/${encodeURIComponent(id)}`;

export function proceduresQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures"],
    queryFn: () => request<Procedure[]>("GET", "/procedures", undefined, { workspace }),
  });
}

export function procedureQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures", id],
    queryFn: () => request<Procedure>("GET", path(id), undefined, { workspace }),
    retry: false,
  });
}

export function createProcedure(workspace: string, name: string): Promise<Procedure> {
  return request<Procedure>("POST", "/procedures", { name }, { workspace });
}

export function saveProcedure(
  workspace: string,
  id: string,
  changes: { name?: string; definition?: ProcedureDefinition; lock_version: number },
): Promise<Procedure> {
  return request<Procedure>("PATCH", path(id), changes, { workspace });
}

export function publishProcedure(workspace: string, id: string): Promise<Procedure> {
  return request<Procedure>("POST", `${path(id)}/publications`, undefined, { workspace });
}

export function unpublishProcedure(workspace: string, id: string): Promise<Procedure> {
  return request<Procedure>("DELETE", `${path(id)}/publications`, undefined, { workspace });
}

export function deleteProcedure(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", path(id), undefined, { workspace });
}
