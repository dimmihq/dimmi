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
  cancelled: "Cancelled",
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
  cancelled: "neutral",
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
export function timeLeft(expiresAt: string | null, now: number): string | null {
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
export function argumentLines(args: Record<string, unknown>): { label: string; value: string }[] {
  return Object.entries(args).map(([key, value]) => ({ label: humanize(key), value: valueText(value) }));
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
  if (typeof call.action === "string") {
    return humanize(call.action);
  }

  return call.action.title || humanize(call.action.name);
}

/*
 * decisionLine is the one line a decided call collapses into on the
 * timeline: "Approved by Ada · Refund issued", "Declined by Ada: not
 * eligible", "Expired: nobody decided in time".
 */
export function decisionLine(call: ActionCall): string {
  const by = call.decided_by?.name;
  const outcome = call.result_summary ? ` · ${call.result_summary}` : "";
  switch (call.state) {
    case "approved":
    case "running":
      return `Approved${by ? ` by ${by}` : ""}${outcome}`;
    case "succeeded":
      return `${by ? `Approved by ${by} · ` : ""}${call.result_summary ?? `${callTitle(call)} done`}`;
    case "failed":
      return `${by ? `Approved by ${by} · ` : ""}Failed${call.result_summary ? `: ${call.result_summary}` : ""}`;
    case "declined":
      return `Declined${by ? ` by ${by}` : ""}${call.decision_reason ? `: ${call.decision_reason}` : ""}`;
    case "expired":
      return "Expired: nobody decided in time";
    case "cancelled":
      return "Cancelled";
    default:
      return callStateLabels[call.state];
  }
}

/*
 * actionCallOf reads the call a message carries, if any, from its
 * metadata.
 */
export function actionCallOf(metadata: unknown): ActionCall | null {
  if (!metadata || typeof metadata !== "object") {
    return null;
  }
  const ai = (metadata as { ai?: unknown }).ai;
  if (!ai || typeof ai !== "object") {
    return null;
  }
  const call = (ai as { action_call?: ActionCall }).action_call;

  return call && typeof call.id === "string" && typeof call.state === "string" ? call : null;
}
