/*
 * The rules the SLA policy screens share: how seconds read as an amount
 * and a unit and back, how a duration reads in a list, the draft a
 * policy's editor keeps, which fields a draft changed, and whether it is
 * ready to save.
 */
import type { SLAPolicy, SLAPolicyChange } from "@/lib/api/sla";

export type DurationUnit = "minutes" | "hours" | "days";

export const durationUnits: DurationUnit[] = ["minutes", "hours", "days"];

const unitSeconds: Record<DurationUnit, number> = { minutes: 60, hours: 3600, days: 86400 };

export interface Duration {
  amount: number;
  unit: DurationUnit;
}

/*
 * toDuration reads seconds in the largest unit that holds them whole.
 */
export function toDuration(seconds: number): Duration {
  for (const unit of [...durationUnits].reverse()) {
    const size = unitSeconds[unit];
    if (seconds >= size && seconds % size === 0) {
      return { amount: seconds / size, unit };
    }
  }

  return { amount: Math.max(1, Math.round(seconds / 60)), unit: "minutes" };
}

export function toSeconds({ amount, unit }: Duration): number {
  return Math.round(amount * unitSeconds[unit]);
}

/*
 * formatDuration reads seconds as days, hours and minutes in the
 * locale's narrow units, such as "1d 4h" or "30m".
 */
export function formatDuration(seconds: number, locale = "en"): string {
  const parts: [number, "day" | "hour" | "minute"][] = [
    [Math.floor(seconds / 86400), "day"],
    [Math.floor((seconds % 86400) / 3600), "hour"],
    [Math.floor((seconds % 3600) / 60), "minute"],
  ];
  const shown = parts.filter(([n]) => n > 0);
  if (shown.length === 0) {
    return new Intl.NumberFormat(locale, { style: "unit", unit: "minute", unitDisplay: "narrow" }).format(0);
  }

  return shown
    .map(([n, unit]) => new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "narrow" }).format(n))
    .join(" ");
}

export type TargetKey = "first_reply" | "next_reply" | "follow_up" | "resolution";

export const targetKeys: TargetKey[] = ["first_reply", "next_reply", "follow_up", "resolution"];

export type BreachPriority = NonNullable<SLAPolicyChange["breach_priority"]>;

export const breachPriorities: BreachPriority[] = ["none", "low", "medium", "high", "urgent"];

/*
 * PolicyDraft is what a policy's editor holds while it changes: targets
 * in seconds, null when off.
 */
export interface PolicyDraft {
  name: string;
  description: string;
  targets: Record<TargetKey, number | null>;
  warn_before: number;
  schedule_id: string | null;
  pause_on_waiting_customer: boolean;
  pause_on_waiting_team: boolean;
  breach_priority: BreachPriority;
  breach_team_id: string | null;
  ticket_type_ids: string[];
  priorities: string[];
}

/*
 * blankDraft is a new policy: a first reply within an hour, warned a
 * quarter of an hour ahead, paused while the customer is away.
 */
export function blankDraft(scheduleId: string | null = null): PolicyDraft {
  return {
    name: "",
    description: "",
    targets: { first_reply: 3600, next_reply: null, follow_up: null, resolution: null },
    warn_before: 900,
    schedule_id: scheduleId,
    pause_on_waiting_customer: true,
    pause_on_waiting_team: false,
    breach_priority: "none",
    breach_team_id: null,
    ticket_type_ids: [],
    priorities: [],
  };
}

export function draftOf(p: SLAPolicy): PolicyDraft {
  return {
    name: p.name,
    description: p.description,
    targets: {
      first_reply: p.first_reply_seconds,
      next_reply: p.next_reply_seconds,
      follow_up: p.follow_up_seconds,
      resolution: p.resolution_seconds,
    },
    warn_before: p.warn_before_seconds,
    schedule_id: p.schedule_id,
    pause_on_waiting_customer: p.pause_on_waiting_customer,
    pause_on_waiting_team: p.pause_on_waiting_team,
    breach_priority: (p.breach_priority ?? "none") as BreachPriority,
    breach_team_id: p.breach_team_id,
    ticket_type_ids: p.ticket_type_ids,
    priorities: p.priorities,
  };
}

/*
 * targetsOf reads a policy's targets by key.
 */
export function targetsOf(p: SLAPolicy): Record<TargetKey, number | null> {
  return draftOf(p).targets;
}

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && [...a].sort().join("\n") === [...b].sort().join("\n");
}

/*
 * policyBody is everything a draft says, in the API's terms: 0 turns a
 * target off, an empty id removes the schedule or team, "none" removes
 * the escalation.
 */
export function policyBody(draft: PolicyDraft): SLAPolicyChange {
  return {
    name: draft.name.trim(),
    description: draft.description.trim(),
    first_reply_seconds: draft.targets.first_reply ?? 0,
    next_reply_seconds: draft.targets.next_reply ?? 0,
    follow_up_seconds: draft.targets.follow_up ?? 0,
    resolution_seconds: draft.targets.resolution ?? 0,
    warn_before_seconds: draft.warn_before,
    schedule_id: draft.schedule_id ?? "",
    pause_on_waiting_customer: draft.pause_on_waiting_customer,
    pause_on_waiting_team: draft.pause_on_waiting_team,
    breach_priority: draft.breach_priority,
    breach_team_id: draft.breach_team_id ?? "",
    ticket_type_ids: draft.ticket_type_ids,
    priorities: draft.priorities,
  };
}

function same(a: unknown, b: unknown): boolean {
  if (Array.isArray(a) && Array.isArray(b)) {
    return sameList(a, b);
  }

  return a === b;
}

/*
 * policyChanges is what the draft changed from the saved policy, or all
 * of it when there is none.
 */
export function policyChanges(saved: PolicyDraft | null, draft: PolicyDraft): SLAPolicyChange {
  const next = policyBody(draft);
  if (saved === null) {
    return next;
  }
  const was: Record<string, unknown> = { ...policyBody(saved) };

  return Object.fromEntries(Object.entries(next).filter(([key, value]) => !same(value, was[key])));
}

/*
 * draftProblem names what keeps a draft from saving: a name, and at
 * least one target. It is null when the draft can save.
 */
export function draftProblem(draft: PolicyDraft): "needsName" | "needsTarget" | null {
  if (draft.name.trim() === "") {
    return "needsName";
  }
  if (targetKeys.every((key) => draft.targets[key] === null)) {
    return "needsTarget";
  }

  return null;
}

/*
 * ordered puts live policies first, in their order, then archived ones.
 */
export function ordered(policies: SLAPolicy[]): SLAPolicy[] {
  return [...policies].sort(
    (a, b) => Number(Boolean(a.archived_at)) - Number(Boolean(b.archived_at)) || a.position - b.position,
  );
}
