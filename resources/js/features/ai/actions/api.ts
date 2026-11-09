/*
 * The agent actions API in the words the actions pages use, over the
 * server's own client in lib/api/ai-actions. The pages edit an input
 * schema as typed rows and a definition without nulls, so those two
 * shapes live here; everything else is the server's type under a short
 * name.
 */
import {
  ACTION_AUTH_TYPES,
  ACTION_CALL_STATES,
  ACTION_CONNECTION_KINDS,
  ACTION_METHODS,
  type Action,
  type ActionApproval,
  type ActionAuthType,
  type ActionCallDecision,
  type ActionCallState,
  type ActionConfirmation,
  type ActionConnectionInput,
  type ActionConnectionKind,
  type ActionConnectionTest,
  type ActionMethod,
  type ActionResponseField,
  type ActionSideEffect,
  type ActionTest,
  type ActionTestContact,
  type ActionTestInput,
  type ActionToolSync,
  actionConnectionsQuery,
  actionVersionsQuery,
  aiActionsKey,
  createActionConnection,
  decideActionCall,
  deleteActionConnection,
  type MessageActionCall,
  type ActionVersion as ServerVersion,
  actionQuery as serverActionQuery,
  actionsQuery as serverActionsQuery,
  createAction as serverCreateAction,
  deleteAction as serverDeleteAction,
  publishAction as serverPublishAction,
  testAction as serverTestAction,
  updateAction as serverUpdateAction,
  syncActionConnectionTools,
  testActionConnection,
  updateActionConnection,
} from "@/lib/api/ai-actions";

export type { ActionConnection, ActionStatus } from "@/lib/api/ai-actions";

export const CONNECTION_KINDS = ACTION_CONNECTION_KINDS;
export const AUTH_TYPES = ACTION_AUTH_TYPES;
export const HTTP_METHODS = ACTION_METHODS;
export const CALL_STATES = ACTION_CALL_STATES;
export const FIELD_TYPES = ["string", "number", "integer", "boolean"] as const;
export const ACTION_CHANNELS = ["widget", "email", "api"] as const;
export const ACTION_AGENTS = ["dimmi", "operator"] as const;

export type ConnectionKind = ActionConnectionKind;
export type AuthType = ActionAuthType;
export type HttpMethod = ActionMethod;
export type CallState = ActionCallState;
export type SideEffect = ActionSideEffect;
export type Confirmation = ActionConfirmation;
export type Approval = ActionApproval;
export type FieldType = (typeof FIELD_TYPES)[number];
export type ActionChannel = (typeof ACTION_CHANNELS)[number];
export type ActionAgent = (typeof ACTION_AGENTS)[number];
export type NewConnection = ActionConnectionInput;
export type ConnectionTest = ActionConnectionTest;
export type ToolSync = ActionToolSync;
export type ResponseField = ActionResponseField;
export type AiAction = Action;
export type ActionVersion = ServerVersion;
export type SampleContact = ActionTestContact;
export type ActionTestRequest = ActionTestInput;
export type ActionTestResult = ActionTest;
export type ActionCall = MessageActionCall;
export type CallDecision = ActionCallDecision;

/*
 * SchemaProperty is one input the agent fills, in the portable subset of
 * JSON Schema every model provider accepts.
 */
export type SchemaProperty = {
  type: FieldType;
  description?: string;
  title?: string;
  enum?: (string | number)[];
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  format?: string;
};

export type InputSchema = {
  type: "object";
  properties: Record<string, SchemaProperty>;
  required?: string[];
  additionalProperties?: false;
};

/*
 * ActionDefinition is what the editor saves: the fields of an action a
 * person edits. A REST action leaves the MCP fields out and an MCP
 * action the request fields, so saving never clears the other kind's.
 */
export type ActionDefinition = {
  connection_id: string;
  name: string;
  title: string;
  description: string;
  side_effect: SideEffect;
  method?: HttpMethod;
  path_template?: string;
  query?: Record<string, string>;
  headers?: Record<string, string>;
  body_template?: unknown;
  body_encoding: "json" | "form";
  mcp_tool_name?: string;
  bound_arguments?: Record<string, string>;
  input_schema: InputSchema;
  response_fields: ResponseField[];
  channels: ActionChannel[];
  agents: ActionAgent[];
  confirmation: Confirmation;
  approval: Approval;
  approval_timeout_minutes: number;
};

export const actionsKey = aiActionsKey;

export function connectionsQuery(workspace: string) {
  return actionConnectionsQuery(workspace);
}

export function actionsQuery(workspace: string) {
  return serverActionsQuery(workspace);
}

export function actionQuery(workspace: string, action: string) {
  return serverActionQuery(workspace, action);
}

export function versionsQuery(workspace: string, action: string) {
  return actionVersionsQuery(workspace, action);
}

export const createConnection = createActionConnection;
export const updateConnection = updateActionConnection;
export const deleteConnection = deleteActionConnection;
export const syncTools = syncActionConnectionTools;
export const decideCall = decideActionCall;

export function testConnection(workspace: string, id: string) {
  return testActionConnection(workspace, id);
}

export function createAction(workspace: string, body: ActionDefinition) {
  return serverCreateAction(workspace, body);
}

export function updateAction(workspace: string, id: string, body: Partial<ActionDefinition>) {
  return serverUpdateAction(workspace, id, body);
}

export function deleteAction(workspace: string, id: string) {
  return serverDeleteAction(workspace, id);
}

export function publishAction(workspace: string, id: string, note?: string) {
  return serverPublishAction(workspace, id, note ? { note } : {});
}

export function restoreVersion(workspace: string, id: string, version: string) {
  return serverUpdateAction(workspace, id, { restore_version_id: version });
}

export function setActionStatus(workspace: string, id: string, status: "published" | "disabled") {
  return serverUpdateAction(workspace, id, { status });
}

export function testAction(workspace: string, id: string, body: ActionTestRequest) {
  return serverTestAction(workspace, id, body);
}
