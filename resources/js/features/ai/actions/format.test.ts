import { describe, expect, it } from "vitest";

import type { ActionCall } from "./api";
import {
  actionCallOf,
  actionLook,
  argumentLines,
  callEventText,
  callOfEvent,
  callTitle,
  connectionLook,
  customerJourney,
  decisionLine,
  durationLabel,
  endpointBase,
  hostOf,
  humanize,
  isOpen,
  permissionSummary,
  requestText,
  responseBody,
  responseStatus,
  syncLine,
  testReason,
  timeLeft,
} from "./format";

const call = (change: Partial<ActionCall> = {}): ActionCall => ({
  id: "aax_1",
  action: { name: "refund_invoice", title: "Refund an invoice" },
  state: "pending_approval",
  summary: "Refund $144.00 for invoice inv_1001",
  arguments: { invoice_id: "inv_1001", amount: 144 },
  expires_at: "2026-11-07T11:00:00Z",
  ...change,
});

describe("action formatting", () => {
  it("shortens addresses to their host", () => {
    expect(hostOf("https://api.example.com/v1/")).toBe("api.example.com");
    expect(hostOf("not a url/path")).toBe("not a url");
    expect(endpointBase("https://api.example.com/v1/")).toBe("api.example.com/v1");
  });

  it("names a connection's status", () => {
    expect(connectionLook({ status: "active", status_reason: null })).toEqual({ label: "Connected", tone: "lime" });
    expect(connectionLook({ status: "error", status_reason: "auth" })).toEqual({ label: "Key refused", tone: "rose" });
    expect(connectionLook({ status: "error", status_reason: null }).label).toBe("Not working");
    expect(connectionLook({ status: "disabled", status_reason: null }).label).toBe("Off");
  });

  it("summarises who must agree before an action runs", () => {
    expect(permissionSummary({ side_effect: "read", confirmation: "customer", approval: "admin" })).toBe(
      "Runs right away",
    );
    expect(permissionSummary({ side_effect: "write", confirmation: "customer", approval: "none" })).toBe(
      "Customer confirms",
    );
    expect(permissionSummary({ side_effect: "write", confirmation: "customer", approval: "teammate" })).toBe(
      "Customer confirms · Teammate approves",
    );
    expect(permissionSummary({ side_effect: "write", confirmation: "none", approval: "admin" })).toBe("Admin approves");
    expect(permissionSummary({ side_effect: "write", confirmation: "none", approval: "none" })).toBe("Runs right away");
  });

  it("says whether an action is live", () => {
    const version = { id: "aacv_1", number: 3, created_at: "" };
    expect(actionLook({ status: "draft", has_unpublished_changes: true, published_version: null }).label).toBe("Draft");
    expect(actionLook({ status: "published", has_unpublished_changes: false, published_version: version })).toEqual({
      label: "Live v3",
      tone: "lime",
    });
    expect(actionLook({ status: "published", has_unpublished_changes: true, published_version: version }).label).toBe(
      "Live v3 · edited",
    );
    expect(actionLook({ status: "disabled", has_unpublished_changes: false, published_version: version }).label).toBe(
      "Off",
    );
  });

  it("writes durations the way people say them", () => {
    expect(durationLabel(15)).toBe("15 min");
    expect(durationLabel(60)).toBe("1 hour");
    expect(durationLabel(240)).toBe("4 hours");
    expect(durationLabel(1440)).toBe("1 day");
    expect(durationLabel(2880)).toBe("2 days");
  });

  it("tells the customer's side of each permission", () => {
    const base = { title: "Refund an invoice", approval_timeout_minutes: 60 };
    expect(customerJourney({ ...base, side_effect: "read", confirmation: "none", approval: "none" })).toHaveLength(2);
    const guarded = customerJourney({ ...base, side_effect: "write", confirmation: "customer", approval: "teammate" });
    expect(guarded[0]).toContain("confirm “Refund an invoice”");
    expect(guarded.join(" ")).toContain("A teammate approves or declines from the inbox within 1 hour");
    expect(guarded.at(-1)).toContain("runs once");
    const direct = customerJourney({ ...base, side_effect: "write", confirmation: "none", approval: "admin" });
    expect(direct[0]).toContain("without asking");
    expect(direct.join(" ")).toContain("An admin approves");
  });

  it("counts down the time a call has left", () => {
    const now = Date.parse("2026-11-07T10:00:00Z");
    expect(timeLeft(null, now)).toBeNull();
    expect(timeLeft("nonsense", now)).toBeNull();
    expect(timeLeft("2026-11-07T09:59:00Z", now)).toBe("Expired");
    expect(timeLeft("2026-11-07T10:00:20Z", now)).toBe("1 min left");
    expect(timeLeft("2026-11-07T10:12:00Z", now)).toBe("12 min left");
    expect(timeLeft("2026-11-07T12:00:00Z", now)).toBe("2 h left");
    expect(timeLeft("2026-11-07T13:05:00Z", now)).toBe("3 h 5 min left");
  });

  it("reads a call's arguments as words", () => {
    expect(humanize("invoice_id")).toBe("Invoice id");
    expect(humanize("orderNumber")).toBe("Order number");
    expect(argumentLines({ invoice_id: "inv_1001", full: true, meta: { a: 1 }, none: null })).toEqual([
      { label: "Invoice id", value: "inv_1001" },
      { label: "Full", value: "Yes" },
      { label: "Meta", value: '{"a":1}' },
      { label: "None", value: "—" },
    ]);
  });

  it("names a call by its action", () => {
    expect(callTitle(call())).toBe("Refund an invoice");
    expect(callTitle(call({ action: undefined }))).toBe("Action");
    expect(callTitle(call({ action: { name: "issue_refund", title: "" } }))).toBe("Issue refund");
  });

  it("collapses a decided call into one line", () => {
    const ada = { id: "mem_1", name: "Ada" };
    expect(decisionLine(call({ state: "approved", decided_by: ada }))).toBe("Approved by Ada");
    expect(decisionLine(call({ state: "succeeded", decided_by: ada, result_summary: "Refund issued $144.00" }))).toBe(
      "Approved by Ada · Refund issued $144.00",
    );
    expect(decisionLine(call({ state: "succeeded" }))).toBe("Done");
    expect(decisionLine(call({ state: "failed", decided_by: ada, result_summary: "already refunded" }))).toBe(
      "Approved by Ada · Failed: already refunded",
    );
    expect(decisionLine(call({ state: "declined", decided_by: ada, decision_reason: "past 30 days" }))).toBe(
      "Declined by Ada: past 30 days",
    );
    expect(decisionLine(call({ state: "expired" }))).toBe("Expired: nobody decided in time");
    expect(decisionLine(call({ state: "canceled" }))).toBe("Canceled");
    expect(decisionLine(call({ state: "running" }))).toBe("Approved");
  });

  it("knows which calls still wait on someone", () => {
    expect(isOpen("awaiting_confirmation")).toBe(true);
    expect(isOpen("pending_approval")).toBe(true);
    expect(isOpen("succeeded")).toBe(false);
  });

  it("reads the call a reply reports", () => {
    expect(actionCallOf(null)).toBeNull();
    expect(actionCallOf({})).toBeNull();
    expect(actionCallOf({ action_call: { id: 3 } })).toBeNull();
    expect(actionCallOf({ action_call: call() })?.id).toBe("aax_1");
  });

  it("turns a call's decision on the timeline into its line", () => {
    const grace = { type: "member", id: "mem_1", name: "Grace" };
    const event = {
      name: "ai.action_call_approved",
      actor: grace,
      call_id: "aax_1",
      action: { id: "aac_1", title: "Refund an invoice" },
      state: "approved" as const,
      summary: "Refund $144.00",
    };
    expect(callOfEvent(event)).toEqual({
      id: "aax_1",
      state: "approved",
      summary: "Refund $144.00",
      action: { name: "", title: "Refund an invoice" },
      decided_by: { id: "mem_1", name: "Grace" },
    });
    const failed = callOfEvent({
      ...event,
      name: "ai.action_call_completed",
      actor: { type: "agent", id: "", name: "dimmi" },
      state: "failed",
      error_code: "upstream_rejected",
    });
    expect(failed && decisionLine(failed)).toBe("Failed: upstream rejected");
    expect(callOfEvent({ ...event, name: "ai.action_call_requested" })).toBeNull();
    expect(callOfEvent({ name: "conversation.closed", actor: grace })).toBeNull();
  });

  it("says a call's request and confirmation in words", () => {
    const action = { id: "aac_1", title: "Refund an invoice" };
    expect(
      callEventText({ name: "ai.action_call_requested", actor: { type: "agent", id: "", name: "Ada" }, action }),
    ).toBe("Ada asked to run Refund an invoice");
    expect(
      callEventText({ name: "ai.action_call_confirmed", actor: { type: "contact", id: "c", name: "" }, action }),
    ).toBe("The customer confirmed Refund an invoice");
    expect(
      callEventText({ name: "ai.action_call_approved", actor: { type: "member", id: "m", name: "G" } }),
    ).toBeNull();
  });

  it("says what a tool sync did and why a test failed", () => {
    expect(syncLine({ created: [], existing: 2, skipped: [] })).toBe("0 new draft actions, 2 already there.");
    expect(
      syncLine({ created: [{} as never], existing: 0, skipped: [{ tool: "export", reason: "schema_unsupported" }] }),
    ).toBe("1 new draft action, 0 already there, 1 skipped: export.");
    expect(testReason("auth")).toBe("The server refused the key.");
    expect(testReason(null)).toBe("The server did not answer.");
  });

  it("writes a test's request and reads its response", () => {
    expect(requestText(null)).toBe("");
    expect(requestText({ tool: "issue_refund", arguments: { a: 1 } })).toContain('"tool": "issue_refund"');
    expect(
      requestText({
        method: "POST",
        url: "https://x.test/refunds",
        headers: { Authorization: "[secret]" },
        body: { a: 1 },
      }),
    ).toBe('POST https://x.test/refunds\nAuthorization: [secret]\n\n{\n  "a": 1\n}');
    expect(requestText({ method: "GET", url: "https://x.test/a" })).toBe("GET https://x.test/a");
    expect(responseBody(null)).toBeUndefined();
    expect(responseBody({ status: 200, body: { ok: true } })).toEqual({ ok: true });
    expect(responseBody({ structured: 1 })).toEqual({ structured: 1 });
    expect(responseStatus({ status: 201 })).toBe(201);
    expect(responseStatus({ status: "x" })).toBeNull();
  });
});
