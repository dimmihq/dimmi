import { describe, expect, it } from "vitest";

import type { ActionCall } from "./api";
import {
  actionCallOf,
  actionLook,
  argumentLines,
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
  timeLeft,
} from "./format";

const call = (change: Partial<ActionCall> = {}): ActionCall => ({
  id: "aax_1",
  action: { name: "refund_invoice", title: "Refund an invoice" },
  state: "pending_approval",
  summary: "Refund $144.00 for invoice inv_1001",
  arguments: { invoice_id: "inv_1001", amount: 144 },
  reason: "The customer bought the annual plan by mistake.",
  expires_at: "2026-11-07T11:00:00Z",
  decided_by: null,
  decision_reason: null,
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
    expect(connectionLook({ status: "error", status_reason: "odd" }).label).toBe("Not working");
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
    expect(callTitle(call({ action: "issue_refund" }))).toBe("Issue refund");
    expect(callTitle(call({ action: { name: "issue_refund", title: null } }))).toBe("Issue refund");
  });

  it("collapses a decided call into one line", () => {
    const ada = { id: "mem_1", name: "Ada" };
    expect(decisionLine(call({ state: "approved", decided_by: ada }))).toBe("Approved by Ada");
    expect(decisionLine(call({ state: "succeeded", decided_by: ada, result_summary: "Refund issued $144.00" }))).toBe(
      "Approved by Ada · Refund issued $144.00",
    );
    expect(decisionLine(call({ state: "succeeded" }))).toBe("Refund an invoice done");
    expect(decisionLine(call({ state: "failed", decided_by: ada, result_summary: "already refunded" }))).toBe(
      "Approved by Ada · Failed: already refunded",
    );
    expect(decisionLine(call({ state: "declined", decided_by: ada, decision_reason: "past 30 days" }))).toBe(
      "Declined by Ada: past 30 days",
    );
    expect(decisionLine(call({ state: "expired" }))).toBe("Expired: nobody decided in time");
    expect(decisionLine(call({ state: "cancelled" }))).toBe("Cancelled");
    expect(decisionLine(call({ state: "running" }))).toBe("Approved");
  });

  it("knows which calls still wait on someone", () => {
    expect(isOpen("awaiting_confirmation")).toBe(true);
    expect(isOpen("pending_approval")).toBe(true);
    expect(isOpen("succeeded")).toBe(false);
  });

  it("reads the call a message carries", () => {
    expect(actionCallOf(null)).toBeNull();
    expect(actionCallOf({ ai: null })).toBeNull();
    expect(actionCallOf({ ai: { action_call: { id: 3 } } })).toBeNull();
    expect(actionCallOf({ ai: { action_call: call() } })?.id).toBe("aax_1");
  });
});
