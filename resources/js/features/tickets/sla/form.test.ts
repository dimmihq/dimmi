import { describe, expect, it } from "vitest";

import type { SLAPolicy } from "@/lib/api/sla";

import {
  blankDraft,
  draftOf,
  draftProblem,
  formatDuration,
  ordered,
  policyBody,
  policyChanges,
  toDuration,
  toSeconds,
} from "./form";

function policy(extra: Partial<SLAPolicy> = {}): SLAPolicy {
  return {
    object: "sla_policy",
    id: "sla_1",
    name: "Enterprise",
    description: "",
    schedule_id: "sch_1",
    first_reply_seconds: 3600,
    next_reply_seconds: null,
    follow_up_seconds: null,
    resolution_seconds: 172800,
    warn_before_seconds: 900,
    pause_on_waiting_customer: true,
    pause_on_waiting_team: false,
    breach_priority: "urgent",
    breach_team_id: null,
    ticket_type_ids: ["tktt_1"],
    priorities: ["high"],
    position: 0,
    archived_at: null,
    ...extra,
  };
}

describe("durations", () => {
  it("reads seconds in the largest whole unit", () => {
    expect(toDuration(3600)).toEqual({ amount: 1, unit: "hours" });
    expect(toDuration(5400)).toEqual({ amount: 90, unit: "minutes" });
    expect(toDuration(172800)).toEqual({ amount: 2, unit: "days" });
    expect(toSeconds({ amount: 3, unit: "days" })).toBe(259200);
  });

  it("formats days, hours and minutes narrowly", () => {
    expect(formatDuration(5400)).toBe("1h 30m");
    expect(formatDuration(97200)).toBe("1d 3h");
    expect(formatDuration(900)).toBe("15m");
  });
});

describe("policyChanges", () => {
  it("sends only what changed, in the API's terms", () => {
    const saved = draftOf(policy());
    const draft = {
      ...saved,
      targets: { ...saved.targets, resolution: null, next_reply: 7200 },
      schedule_id: null,
      breach_priority: "none" as const,
      ticket_type_ids: [],
    };

    expect(policyChanges(saved, draft)).toEqual({
      next_reply_seconds: 7200,
      resolution_seconds: 0,
      schedule_id: "",
      breach_priority: "none",
      ticket_type_ids: [],
    });
  });

  it("ignores the order of picked lists", () => {
    const saved = draftOf(policy({ priorities: ["high", "urgent"] }));

    expect(policyChanges(saved, { ...saved, priorities: ["urgent", "high"] })).toEqual({});
  });

  it("sends a whole new policy", () => {
    const body = policyBody({ ...blankDraft("sch_1"), name: " Standard " });

    expect(body).toMatchObject({ name: "Standard", first_reply_seconds: 3600, resolution_seconds: 0 });
    expect(body.schedule_id).toBe("sch_1");
  });
});

describe("draftProblem", () => {
  it("asks for a name and a target", () => {
    expect(draftProblem(blankDraft())).toBe("needsName");
    const named = { ...blankDraft(), name: "Gold" };
    expect(draftProblem(named)).toBeNull();
    expect(
      draftProblem({
        ...named,
        targets: { first_reply: null, next_reply: null, follow_up: null, resolution: null },
      }),
    ).toBe("needsTarget");
  });
});

it("puts archived policies last", () => {
  const list = ordered([
    policy({ id: "a", archived_at: "2026-01-01T00:00:00Z", position: 0 }),
    policy({ id: "b", position: 2 }),
    policy({ id: "c", position: 1 }),
  ]);

  expect(list.map((p) => p.id)).toEqual(["c", "b", "a"]);
});
