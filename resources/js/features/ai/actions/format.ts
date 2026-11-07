/*
 * How actions and their calls read in words: what an action may do and
 * who must agree first, the plain story of what a customer goes through
 * when the agent uses it, where a call stands and how long it has left,
 * and a call's arguments as a teammate reads them. Nothing here talks to
 * the server, so all of it is tested on its own.
 */
import type {
  ActionCall,
  ActionConnection,
  AiAction,
  Approval,
  AuthType,
  CallState,
  Confirmation,
  SideEffect,
  ToolSync,
} from "./api";

export type Tone = "lime" | "rose" | "neutral" | "yellow" | "blue" | "purple" | "orange";

/*
 * hostOf is the host of an address, or the address itself when it does
 * not parse, so a list row stays short.
 */
export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url.replace(/^https?:\/\//, "").split("/")[0] ?? url;
  }
}

/*
 * endpointBase is a base URL without its scheme or trailing slash, as
 * the start of every action's path: api.example.com/v1.
 */
export function endpointBase(url: string): string {
  return url.replace(/^https?:\/\//, "").replace(/\/+$/, "");
}

export const authLabels: Record<AuthType, string> = {
  none: "No auth",
  bearer: "Bearer token",
  api_key_header: "API key in a header",
  api_key_query: "API key in the query",
  basic: "Username and password",
};

/*
 * connectionLook is the chip of a connection's status: Connected, a
 * failure with its reason, or Off.
 */
export function connectionLook(connection: Pick<ActionConnection, "status" | "status_reason">): {
  label: string;
  tone: Tone;
} {
  if (connection.status === "disabled") {
    return { label: "Off", tone: "neutral" };
  }
  if (connection.status === "error") {
    return { label: reasonLabels[connection.status_reason ?? ""] ?? "Not working", tone: "rose" };
  }

  return { label: "Connected", tone: "lime" };
}

const reasonLabels: Record<string, string> = {
  auth: "Key refused",
  tls: "Certificate problem",
  unreachable: "Unreachable",
  host_blocked: "Address blocked",
  mcp_incompatible: "Not an MCP server",
};

/*
 * requestText writes a test's request as it went out: the method, the
 * address, the headers with secrets masked and the body, or for an MCP
 * tool its name and arguments.
 */
export function requestText(request: Record<string, unknown> | null): string {
  if (!request) {
    return "";
  }
  if (typeof request.method !== "string") {
    return JSON.stringify(request, null, 2);
  }
  const headers = Object.entries((request.headers ?? {}) as Record<string, unknown>).map(
    ([name, value]) => `${name}: ${String(value)}`,
  );
  const body = request.body === null || request.body === undefined ? [] : ["", JSON.stringify(request.body, null, 2)];

  return [`${request.method} ${String(request.url ?? "")}`, ...headers, ...body].join("\n");
}

/*
 * responseBody is what an API answered a test, or undefined when nothing
 * was sent.
 */
export function responseBody(response: Record<string, unknown> | null | undefined): unknown {
  if (!response) {
    return undefined;
  }

  return "body" in response ? response.body : response;
}

/*
 * responseStatus is the HTTP status a test's API answered with.
 */
export function responseStatus(response: Record<string, unknown> | null | undefined): number | null {
  return typeof response?.status === "number" ? response.status : null;
}

const testReasons: Record<string, string> = {
  auth: "The server refused the key.",
  credentials: "The secret is missing or cannot be read. Paste it again.",
  unreachable: "The server could not be reached.",
  host_blocked: "dimmi may not call this address.",
  url_escape: "The address leaves the connection's base URL.",
};

/*
 * testReason says why a connection test failed.
 */
export function testReason(reason: string | null): string {
  return testReasons[reason ?? ""] ?? "The server did not answer.";
}

/*
 * syncLine says what a tool sync did: "3 new draft actions, 2 already
 * there", with the tools it could not use.
 */
export function syncLine(sync: Pick<ToolSync, "created" | "existing" | "skipped">): string {
  const created = sync.created.length;
  const parts = [
    created === 1 ? "1 new draft action" : `${created} new draft actions`,
    `${sync.existing} already there`,
  ];
  if (sync.skipped.length > 0) {
    parts.push(`${sync.skipped.length} skipped: ${sync.skipped.map((s) => s.tool).join(", ")}`);
  }

  return `${parts.join(", ")}.`;
}

export const sideEffectLabels: Record<SideEffect, string> = { read: "Reads", write: "Changes data" };

/*
 * permissionSummary says who must agree before the action runs, as its
 * list row shows it: "Runs right away", "Customer confirms" or
 * "Customer confirms · Teammate approves".
 */
export function permissionSummary(action: {
  side_effect: SideEffect;
  confirmation: Confirmation;
  approval: Approval;
}): string {
  if (action.side_effect === "read") {
    return "Runs right away";
  }
  const parts: string[] = [];
  if (action.confirmation === "customer") {
    parts.push("Customer confirms");
  }
  if (action.approval !== "none") {
    parts.push(action.approval === "admin" ? "Admin approves" : "Teammate approves");
  }

  return parts.length > 0 ? parts.join(" · ") : "Runs right away";
}

/*
 * actionLook is the chip of an action's state: a draft never published,
 * live with unpublished edits, live, or off.
 */
export function actionLook(action: Pick<AiAction, "status" | "has_unpublished_changes" | "published_version">): {
  label: string;
  tone: Tone;
} {
  if (action.status === "disabled") {
    return { label: "Off", tone: "neutral" };
  }
  if (action.status === "draft" || !action.published_version) {
    return { label: "Draft", tone: "neutral" };
  }
  if (action.has_unpublished_changes) {
    return { label: `Live v${action.published_version.number} · edited`, tone: "yellow" };
  }

  return { label: `Live v${action.published_version.number}`, tone: "lime" };
}

/*
 * durationLabel writes minutes the way people say them: 45 min, 2 hours,
 * 1 day.
 */
export function durationLabel(minutes: number): string {
  if (minutes < 60) {
    return `${minutes} min`;
  }
  if (minutes < 1440) {
    const hours = Math.round(minutes / 60);
    return hours === 1 ? "1 hour" : `${hours} hours`;
  }
  const days = Math.round(minutes / 1440);

  return days === 1 ? "1 day" : `${days} days`;
}

/*
 * customerJourney is what the customer goes through when the agent uses
 * the action, step by step, so an admin sees the effect of the
 * permissions they pick before publishing.
 */
export function customerJourney(action: {
  side_effect: SideEffect;
  confirmation: Confirmation;
  approval: Approval;
  approval_timeout_minutes: number;
  title: string;
}): string[] {
  const what = action.title.trim() || "the action";
  if (action.side_effect === "read") {
    return [
      `dimmi looks it up while it answers, with “${what}”.`,
      "The customer only sees the answer. Nothing changes in your systems.",
    ];
  }
  const steps: string[] = [];
  if (action.confirmation === "customer") {
    steps.push(`dimmi shows a card asking the customer to confirm “${what}”, with exactly what will happen.`);
    steps.push("Nothing runs until they press Confirm. Cancel stops it.");
  } else {
    steps.push(`dimmi decides to use “${what}” without asking the customer first.`);
  }
  if (action.approval !== "none") {
    const who = action.approval === "admin" ? "An admin" : "A teammate";
    steps.push(
      `dimmi tells the customer the team is checking. ${who} approves or declines from the inbox within ${durationLabel(action.approval_timeout_minutes)}.`,
    );
    steps.push("If nobody decides in time, the request expires and the conversation goes to the team.");
  }
  steps.push("The action runs once, and dimmi tells the customer how it went.");

  return steps;
}

export const callStateLabels: Record<CallState, string> = {
  awaiting_confirmation: "Waiting for the customer",
  pending_approval: "Waiting for approval",
  approved: "Approved",
  declined: "Declined",
  expired: "Expired",
  canceled: "Canceled",
  running: "Running",
  succeeded: "Done",
  failed: "Failed",
};

export const callStateTones: Record<CallState, Tone> = {
  awaiting_confirmation: "blue",
  pending_approval: "orange",
  approved: "lime",
  declined: "rose",
  expired: "neutral",
  canceled: "neutral",
  running: "blue",
  succeeded: "lime",
  failed: "rose",
};

/*
 * isOpen says a call still waits on someone: the customer or the team.
 */
export function isOpen(state: CallState): boolean {
  return state === "awaiting_confirmation" || state === "pending_approval";
}

/*
 * timeLeft is how long a call has before it expires, in words: "12 min
 * left", "3 h 5 min left", or "Expired" once the time is up.
 */
export function timeLeft(expiresAt: string | null | undefined, now: number): string | null {
  if (!expiresAt) {
    return null;
  }
  const ms = Date.parse(expiresAt) - now;
  if (Number.isNaN(ms)) {
    return null;
  }
  if (ms <= 0) {
    return "Expired";
  }
  const minutes = Math.max(1, Math.ceil(ms / 60000));
  if (minutes < 60) {
    return `${minutes} min left`;
  }
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;

  return rest === 0 ? `${hours} h left` : `${hours} h ${rest} min left`;
}

/*
 * humanize turns an argument's key into words: invoice_id becomes
 * "Invoice id".
 */
export function humanize(key: string): string {
  const words = key
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .trim()
    .toLowerCase();

  return words.charAt(0).toUpperCase() + words.slice(1);
}

/*
 * argumentLines lists a call's frozen arguments as label and value, for
 * a teammate to read before deciding.
 */
export function argumentLines(args: Record<string, unknown> | undefined): { label: string; value: string }[] {
  return Object.entries(args ?? {}).map(([key, value]) => ({ label: humanize(key), value: valueText(value) }));
}

function valueText(value: unknown): string {
  if (value === null || value === undefined) {
    return "—";
  }
  if (typeof value === "boolean") {
    return value ? "Yes" : "No";
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }

  return String(value);
}

/*
 * callTitle is the name a call shows: the action's title when it has
 * one, else its tool name in words.
 */
export function callTitle(call: Pick<ActionCall, "action">): string {
  if (!call.action) {
    return "Action";
  }

  return call.action.title || humanize(call.action.name);
}

function by(call: ActionCall): string {
  return call.decided_by?.name ? ` by ${call.decided_by.name}` : "";
}

function approvedBy(call: ActionCall): string {
  return call.decided_by?.name ? `Approved by ${call.decided_by.name} · ` : "";
}

/*
 * decisionLine is the one line a decided call collapses into on the
 * timeline: "Approved by Ada · Refund issued", "Declined by Ada: not
 * eligible", "Expired: nobody decided in time".
 */
export function decisionLine(call: ActionCall): string {
  const outcome = call.result_summary ? ` · ${call.result_summary}` : "";
  switch (call.state) {
    case "approved":
    case "running":
      return `Approved${by(call)}${outcome}`;
    case "succeeded":
      return `${approvedBy(call)}${call.result_summary ?? "Done"}`;
    case "failed":
      return `${approvedBy(call)}Failed${call.result_summary ? `: ${call.result_summary}` : ""}`;
    case "declined":
      return `Declined${by(call)}${call.decision_reason ? `: ${call.decision_reason}` : ""}`;
    case "expired":
      return "Expired: nobody decided in time";
    case "canceled":
      return "Canceled";
    default:
      return callStateLabels[call.state];
  }
}

/*
 * actionCallOf reads the call an agent reply reports, if any, from the
 * reply's agent mark.
 */
export function actionCallOf(ai: unknown): ActionCall | null {
  if (!ai || typeof ai !== "object") {
    return null;
  }
  const call = (ai as { action_call?: ActionCall }).action_call;

  return call && typeof call.id === "string" && typeof call.state === "string" ? call : null;
}

/*
 * CallEvent is a timeline entry recording one step of a call: who took
 * it, the call, its action, where it then stood and, for a failure, why.
 */
export interface CallEvent {
  name: string;
  actor: { type: string; id: string; name: string };
  call_id?: string;
  action?: { id: string; title: string };
  state?: CallState;
  summary?: string;
  error_code?: string | null;
}

const decidedEvents = [
  "ai.action_call_approved",
  "ai.action_call_declined",
  "ai.action_call_canceled",
  "ai.action_call_expired",
  "ai.action_call_completed",
];

/*
 * callOfEvent turns a call's decision or outcome on the timeline into
 * the call its line shows, with a teammate who decided as its decider
 * and a failure's code as its outcome. Other entries answer null.
 */
export function callOfEvent(event: CallEvent): ActionCall | null {
  if (!decidedEvents.includes(event.name) || !event.call_id || !event.state) {
    return null;
  }
  const call: ActionCall = {
    id: event.call_id,
    state: event.state,
    summary: event.summary ?? "",
    action: { name: "", title: event.action?.title ?? "" },
  };
  if (event.actor.type === "member" && event.actor.name) {
    call.decided_by = { id: event.actor.id, name: event.actor.name };
  }
  if (event.state === "failed" && event.error_code) {
    call.result_summary = humanize(event.error_code).toLowerCase();
  }

  return call;
}

/*
 * callEventText says in words a call's request or the customer's
 * confirmation: "dimmi asked to run Refund an invoice".
 */
export function callEventText(event: CallEvent): string | null {
  const title = event.action?.title || "an action";
  if (event.name === "ai.action_call_requested") {
    return `${event.actor.name || "dimmi"} asked to run ${title}`;
  }
  if (event.name === "ai.action_call_confirmed") {
    return `${event.actor.type === "contact" ? "The customer" : event.actor.name || "The customer"} confirmed ${title}`;
  }

  return null;
}
