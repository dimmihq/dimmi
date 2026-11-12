/*
 * What a macro does to a conversation once its reply is sent, as the
 * conversation's own update takes it, and how the screen names it. A
 * send that itself closes or snoozes keeps that choice: the macro's
 * status gives way to it.
 */
import type { ConversationChanges } from "@/lib/api/inbox";
import type { SavedReplyActions, SnoozePreset } from "@/lib/api/saved-replies";

export const snoozeLabels: Record<SnoozePreset, string> = {
  later_today: "later today",
  tomorrow: "tomorrow",
  next_week: "next week",
  one_week: "in a week",
};

const priorityNames: Record<string, string> = {
  none: "No priority",
  low: "Low priority",
  medium: "Medium priority",
  high: "High priority",
  urgent: "Urgent",
};

/*
 * hasActions reports whether the reply does anything beyond its text.
 */
export function hasActions(actions: SavedReplyActions | undefined): actions is SavedReplyActions {
  if (!actions) {
    return false;
  }

  return Boolean(
    actions.status || actions.assignee_id || actions.team_id || actions.priority || actions.tag_ids.length > 0,
  );
}

/*
 * macroChanges turns the actions into the conversation's update, or null
 * when there is nothing to change. keepStatus leaves the status to the
 * send that already closed or snoozed.
 */
export function macroChanges(actions: SavedReplyActions, keepStatus = false): ConversationChanges | null {
  const changes: ConversationChanges = {};
  if (actions.status && !keepStatus) {
    changes.status = actions.status;
    if (actions.status === "snoozed") {
      changes.snooze_preset = actions.snooze_preset ?? "tomorrow";
    }
  }
  if (actions.assignee_id) {
    changes.assignee_id = actions.assignee_id;
  }
  if (actions.team_id) {
    changes.team_id = actions.team_id;
  }
  if (actions.priority) {
    changes.priority = actions.priority;
  }
  if (actions.tag_ids.length > 0) {
    changes.add_tags = actions.tag_ids;
  }

  return Object.keys(changes).length > 0 ? changes : null;
}

/*
 * Names says what an id is called, for teammates, teams and tags.
 */
export type Names = (id: string) => string | undefined;

/*
 * macroSummary lists what the actions do in a few words each, such as
 * "Close" or "Assign to Ada".
 */
export function macroSummary(actions: SavedReplyActions, names: Names = () => undefined): string[] {
  const out: string[] = [];
  if (actions.status === "closed") {
    out.push("Close");
  } else if (actions.status === "snoozed") {
    out.push(`Snooze until ${snoozeLabels[actions.snooze_preset ?? "tomorrow"]}`);
  } else if (actions.status === "open") {
    out.push("Reopen");
  }
  if (actions.assignee_id) {
    out.push(`Assign to ${names(actions.assignee_id) ?? "a teammate"}`);
  }
  if (actions.team_id) {
    out.push(`Assign to ${names(actions.team_id) ?? "a team"}`);
  }
  for (const tag of actions.tag_ids) {
    out.push(`Tag ${names(tag) ?? "a tag"}`);
  }
  if (actions.priority) {
    out.push(priorityNames[actions.priority] ?? actions.priority);
  }

  return out;
}
