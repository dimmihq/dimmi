/*
 * The macro part of a saved reply: what happens to the conversation once
 * the reply is sent. Every choice is picked from what the workspace has,
 * its statuses, teammates, teams, tags and priorities, never typed.
 */
import { useQuery } from "@tanstack/react-query";

import { Label } from "@/components/base/input/label";
import { Select, SelectItem } from "@/components/base/select/select";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import type { ConversationPriority } from "@/lib/api/inbox";
import type { SavedReplyActions, SnoozePreset } from "@/lib/api/saved-replies";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { snoozeLabels } from "./macro";

const none = "none";

type StatusChoice = "none" | "open" | "closed" | SnoozePreset;

const statusChoices: { id: StatusChoice; label: string }[] = [
  { id: "none", label: "Leave as it is" },
  { id: "closed", label: "Close" },
  ...(Object.entries(snoozeLabels) as [SnoozePreset, string][]).map(([id, label]) => ({
    id,
    label: `Snooze until ${label}`,
  })),
  { id: "open", label: "Reopen" },
];

const priorities: { id: ConversationPriority | "none"; label: string }[] = [
  { id: "none", label: "Leave as it is" },
  { id: "low", label: "Low" },
  { id: "medium", label: "Medium" },
  { id: "high", label: "High" },
  { id: "urgent", label: "Urgent" },
];

function statusOf(actions: SavedReplyActions): StatusChoice {
  if (actions.status === "snoozed") {
    return actions.snooze_preset ?? "tomorrow";
  }

  return actions.status ?? "none";
}

function withStatus(actions: SavedReplyActions, choice: StatusChoice): SavedReplyActions {
  const { status: _status, snooze_preset: _preset, ...rest } = actions;
  if (choice === "none") {
    return rest;
  }
  if (choice === "open" || choice === "closed") {
    return { ...rest, status: choice };
  }

  return { ...rest, status: "snoozed", snooze_preset: choice };
}

/*
 * pick sets one field of the actions, leaving it out when nothing is
 * picked.
 */
function pick<K extends "assignee_id" | "team_id" | "priority">(
  actions: SavedReplyActions,
  key: K,
  value: string,
): SavedReplyActions {
  const next = { ...actions };
  if (value === none) {
    delete next[key];
  } else {
    (next as Record<string, unknown>)[key] = value;
  }

  return next;
}

export function ActionsPicker({
  workspace,
  value,
  onChange,
}: {
  workspace: string;
  value: SavedReplyActions;
  onChange: (next: SavedReplyActions) => void;
}) {
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const { data: tags = [] } = useQuery(tagsQuery(workspace));
  const active = members.filter((member) => member.status === "active");
  const toggleTag = (id: string) =>
    onChange({
      ...value,
      tag_ids: value.tag_ids.includes(id) ? value.tag_ids.filter((tag) => tag !== id) : [...value.tag_ids, id],
    });

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Select
          label="Status"
          selectedKey={statusOf(value)}
          onSelectionChange={(key) => onChange(withStatus(value, String(key) as StatusChoice))}
        >
          {statusChoices.map((choice) => (
            <SelectItem key={choice.id} id={choice.id} textValue={choice.label}>
              {choice.label}
            </SelectItem>
          ))}
        </Select>
        <Select
          label="Priority"
          selectedKey={value.priority ?? none}
          onSelectionChange={(key) => onChange(pick(value, "priority", String(key)))}
        >
          {priorities.map((choice) => (
            <SelectItem key={choice.id} id={choice.id} textValue={choice.label}>
              {choice.label}
            </SelectItem>
          ))}
        </Select>
        <Select
          label="Assign to teammate"
          selectedKey={value.assignee_id ?? none}
          onSelectionChange={(key) => onChange(pick(value, "assignee_id", String(key)))}
        >
          {[{ id: none, name: "Nobody new" }, ...active.map((m) => ({ id: m.id, name: m.user.name }))].map((m) => (
            <SelectItem key={m.id} id={m.id} textValue={m.name}>
              {m.name}
            </SelectItem>
          ))}
        </Select>
        <Select
          label="Assign to team"
          selectedKey={value.team_id ?? none}
          onSelectionChange={(key) => onChange(pick(value, "team_id", String(key)))}
        >
          {[{ id: none, name: "No team change" }, ...teams].map((team) => (
            <SelectItem key={team.id} id={team.id} textValue={team.name}>
              {team.name}
            </SelectItem>
          ))}
        </Select>
      </div>
      <div className="flex flex-col gap-1.5">
        <Label>Add tags</Label>
        {tags.length === 0 ? (
          <p className="text-body-2-regular text-text-tertiary">The workspace has no tags yet.</p>
        ) : (
          <fieldset aria-label="Add tags" className="m-0 flex flex-wrap gap-1.5 border-0 p-0">
            {tags.map((tag) => {
              const on = value.tag_ids.includes(tag.id);
              return (
                <button
                  key={tag.id}
                  type="button"
                  aria-pressed={on}
                  onClick={() => toggleTag(tag.id)}
                  disabled={!on && value.tag_ids.length >= 10}
                  className={cx(
                    "cursor-pointer rounded-full outline-none ring-offset-1 transition-opacity focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-not-allowed",
                    on ? "ring-2 ring-border-focus-ring" : "opacity-60 hover:opacity-100",
                  )}
                >
                  <TagChip tag={tag} />
                </button>
              );
            })}
          </fieldset>
        )}
      </div>
    </div>
  );
}
