/*
 * What a simulated workflow did to the conversation, worded the way the
 * inbox words the same change, and where in the chat each one happened.
 */
import { useQuery } from "@tanstack/react-query";

import { useAccount } from "@/features/settings/use-account";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import type { ChatLine, DryAction, DryTrace } from "@/lib/api/workflow-simulations";
import { membersQuery } from "@/lib/api/workspace";

/*
 * actor is who the inbox names for what a workflow does.
 */
export const actor = "Workflow";

/*
 * Names reads the names behind the ids an action carries.
 */
export interface Names {
  tag: (id: string) => string;
  team: (id: string) => string;
  member: (id: string) => string;
}

/*
 * useNames reads the workspace's tags, teams and teammates by id.
 */
export function useNames(): Names {
  const workspace = useAccount().workspace.id;
  const tags = useQuery(tagsQuery(workspace)).data ?? [];
  const teams = useQuery(teamsQuery(workspace)).data ?? [];
  const members = useQuery(membersQuery(workspace)).data ?? [];

  return {
    tag: (id) => tags.find((t) => t.id === id)?.name ?? "a tag",
    team: (id) => teams.find((t) => t.id === id)?.name ?? "a team",
    member: (id) => members.find((m) => m.id === id)?.user.name ?? "a teammate",
  };
}

export type Moment = { kind: "line"; line: ChatLine; index: number } | { kind: "action"; action: DryAction };

/*
 * moments puts the workflow's actions into the chat: each action shows
 * before the first line of a later step, so it sits where it happened.
 */
export function moments(lines: ChatLine[], trace?: DryTrace): Moment[] {
  const actions = trace?.actions ?? [];
  if (actions.length === 0) {
    return lines.map((line, index) => ({ kind: "line", line, index }));
  }
  const order = new Map<string, number>();
  for (const [at, step] of (trace?.steps ?? []).entries()) {
    if (!order.has(step.step_id)) {
      order.set(step.step_id, at);
    }
  }
  const place = (id: string) => order.get(id) ?? Number.MAX_SAFE_INTEGER;
  const queue = [...actions];
  const out: Moment[] = [];
  const flush = (before: number) => {
    while (queue.length > 0 && place(queue[0]?.step_id ?? "") < before) {
      const action = queue.shift();
      if (action) {
        out.push({ kind: "action", action });
      }
    }
  };
  for (const [index, line] of lines.entries()) {
    if (line.step_id) {
      flush(place(line.step_id));
    }
    out.push({ kind: "line", line, index });
  }
  flush(Number.MAX_SAFE_INTEGER);

  return out;
}

const list = (value: unknown): string[] => (Array.isArray(value) ? value.map(String) : []);
const one = (value: unknown): string => (typeof value === "string" ? value : "");

/*
 * eventText words an action the way the inbox words the same change.
 */
export function eventText(action: DryAction, names: Names): string {
  const d = action.detail ?? {};
  switch (action.kind) {
    case "tag":
      return `${actor} tagged the conversation ${list(d.tag_ids).map(names.tag).join(", ")}`;
    case "untag":
      return `${actor} removed the tag ${list(d.tag_ids).map(names.tag).join(", ")}`;
    case "priority":
      return one(d.priority) === "none"
        ? `${actor} removed the priority`
        : `${actor} set the priority to ${one(d.priority)}`;
    case "assign": {
      const to = one(d.member_id) ? names.member(one(d.member_id)) : names.team(one(d.team_id));
      return `${actor} assigned the conversation to ${to}`;
    }
    case "unassign":
      return `${actor} unassigned the conversation`;
    case "close":
      return `${actor} closed the conversation`;
    case "snooze":
      return `${actor} snoozed the conversation`;
  }
  const said = action.summary.replace(/^Would /, "");

  return `${actor}: ${said.charAt(0).toLowerCase()}${said.slice(1)}${action.mocked ? " (mocked)" : ""}`;
}
