/*
 * The editor of a saved view, opened from the inbox header in a
 * dropdown: its name, who sees it, and its conditions in the same
 * builder as audiences, groups as cards. Within a group the conditions
 * all apply, or any of them; the groups join the same way. A new view
 * can start from the list on screen, keeping its status and sort, and
 * from the conditions already applied to it. Given onApply, its main
 * button applies the conditions to the list without saving anything,
 * and saving it as a view, which then opens, sits behind the button's
 * other half and needs a name. Without it the editor only saves.
 */
import { RiAddLine, RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { PopoverFooter, PopoverHeader } from "@/components/base/popover/popover";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { CardJoin, MatchPick, RuleCard } from "@/features/audiences/rule-layout";
import { joinWord, type Match } from "@/features/audiences/rules";
import { SplitButton } from "@/features/shell/split-button";
import { ApiError } from "@/lib/api/client";
import { createView, deleteView, type SavedView, updateView, type ViewDefinition } from "@/lib/api/views";
import { callerQuery } from "@/lib/api/workspace";
import { realtimeKeys } from "@/lib/realtime/keys";

import { ConditionRow } from "./condition-row";
import { blankDefinition, type KeyedGroup, keyed, keyedCondition, unkeyed, warned } from "./fields";
import { hasConditions } from "./where";

const scopes = { personal: "personal", shared: "shared" } as const;

const views: Match[] = ["all", "any"];

function matchOf(op: "and" | "or"): Match {
  return op === "or" ? "any" : "all";
}

function opOf(match: Match): "and" | "or" {
  return match === "any" ? "or" : "and";
}

export interface ViewDraftStart {
  sort?: string;
  status?: string;
}

function GroupEditor({
  workspace,
  index,
  group,
  only,
  warnings,
  onChange,
  onRemove,
}: {
  workspace: string;
  index: number;
  group: KeyedGroup;
  only: boolean;
  warnings: SavedView["warnings"];
  onChange: (group: KeyedGroup) => void;
  onRemove: () => void;
}) {
  const match = matchOf(group.op);

  return (
    <RuleCard
      index={index}
      match={match}
      choices={views}
      only={only}
      onMatch={(next) => onChange({ ...group, op: opOf(next) })}
      onAdd={() => onChange({ ...group, conditions: [...group.conditions, keyedCondition()] })}
      onRemove={onRemove}
    >
      {group.conditions.map((condition, i) => (
        <ConditionRow
          key={condition.key}
          workspace={workspace}
          join={joinWord(match, i)}
          condition={condition}
          flagged={warned(warnings, index, i)}
          onChange={(next) =>
            onChange({
              ...group,
              conditions: group.conditions.map((c, j) => (j === i ? { ...next, key: c.key } : c)),
            })
          }
          onRemove={() => onChange({ ...group, conditions: group.conditions.filter((_, j) => j !== i) })}
        />
      ))}
    </RuleCard>
  );
}

const headings = {
  edit: { title: "Edit view", description: "Conversations that match, listed live.", name: undefined },
  filter: {
    title: "Filter conversations",
    description: "Apply to this list now, or save as a view to keep.",
    name: "Only needed to save it",
  },
  create: { title: "New view", description: "Conversations that match, listed live.", name: undefined },
};

/*
 * headingOf says what the editor is doing: changing a saved view,
 * filtering the list, or making a new view.
 */
function headingOf(view: SavedView | null, onApply: unknown): keyof typeof headings {
  if (view) {
    return "edit";
  }

  return onApply ? "filter" : "create";
}

/*
 * EditorFooter finishes the editor. A saved view is saved or deleted; a
 * new one is applied to the list, or saved as a view once it has a name.
 */
function EditorFooter({
  editing,
  named,
  filtering,
  saving,
  onDelete,
  onCancel,
  onSave,
  onApply,
}: {
  editing: boolean;
  named: boolean;
  filtering: boolean;
  saving: boolean;
  onDelete: () => void;
  onCancel: () => void;
  onSave: () => void;
  onApply?: () => void;
}) {
  return (
    <PopoverFooter className="border-separator-border border-t px-1.5 pt-3 pb-1.5">
      {editing ? (
        <Button variant="danger-secondary" leadingIcon={RiDeleteBinLine} onClick={onDelete} className="me-auto">
          Delete
        </Button>
      ) : null}
      <Button variant="secondary" onClick={onCancel}>
        Cancel
      </Button>
      {onApply ? (
        <SplitButton
          label="Apply"
          menuLabel="More ways to finish"
          disabled={!filtering}
          loading={saving}
          onPress={onApply}
          choices={[
            {
              label: "Save as view",
              detail: named ? "Keep it in your views and open it." : "Name it above to save it.",
              disabled: !named,
              onSelect: onSave,
            },
          ]}
        />
      ) : (
        <Button onClick={onSave} loading={saving} disabled={!named}>
          Save view
        </Button>
      )}
    </PopoverFooter>
  );
}

export function ViewEditor({
  workspace,
  view,
  start,
  applied = null,
  onApply,
  onClose,
}: {
  workspace: string;
  view: SavedView | null;
  start?: ViewDraftStart;
  applied?: ViewDefinition | null;
  onApply?: (definition: ViewDefinition) => void;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const canShare = caller?.member?.permissions.includes("views:manage_shared") ?? false;
  const [name, setName] = useState(view?.name ?? "");
  const [scope, setScope] = useState<SavedView["scope"]>(view?.scope ?? "personal");
  const [definition, setDefinition] = useState(() => keyed(view?.definition ?? applied ?? blankDefinition()));
  const save = useMutation({
    mutationFn: () =>
      view
        ? updateView(workspace, view.id, { name, definition: unkeyed(definition) })
        : createView(workspace, {
            name,
            scope,
            definition: unkeyed(definition),
            sort: start?.sort,
            status_filter: start?.status,
          }),
    onSuccess: (saved) => {
      void client.invalidateQueries({ queryKey: realtimeKeys.views(workspace) });
      void client.invalidateQueries({ queryKey: realtimeKeys.conversations(workspace) });
      onClose();
      void navigate({ to: "/d/inbox/$view", params: { view: saved.id } });
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteView(workspace, view?.id ?? ""),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: realtimeKeys.views(workspace) });
      onClose();
      void navigate({ to: "/d/inbox/$view", params: { view: "all" } });
    },
  });
  const setGroup = (index: number, group: KeyedGroup | null) =>
    setDefinition({
      ...definition,
      groups: group
        ? definition.groups.map((g, i) => (i === index ? group : g))
        : definition.groups.filter((_, i) => i !== index),
    });
  const refusal = save.error instanceof ApiError ? save.error.message : null;
  const heading = headings[headingOf(view, onApply)];

  return (
    <>
      <PopoverHeader
        title={view ? "Edit view" : onApply ? "Filter conversations" : "New view"}
        description={
          onApply && !view
            ? "Apply to this list now, or save as a view to keep."
            : "Conversations that match, listed live."
        }
        className="px-1.5 pt-1.5"
      />
      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-1.5 py-1">
        <div className="flex flex-wrap items-end gap-3">
          <Input
            label="Name"
            placeholder={heading.name}
            value={name}
            onChange={setName}
            maxLength={60}
            className="min-w-48 flex-1"
          />
          {view === null && canShare ? (
            <SegmentedControl
              aria-label="Who sees it"
              selectedKeys={[scope]}
              onSelectionChange={(selected) => setScope(selected.has("shared") ? "shared" : "personal")}
            >
              <SegmentedControlItem id={scopes.personal}>Only me</SegmentedControlItem>
              <SegmentedControlItem id={scopes.shared}>Everyone</SegmentedControlItem>
            </SegmentedControl>
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="text-body-medium text-text-secondary">Conversations match</span>
          <MatchPick
            label="How groups match"
            value={matchOf(definition.op)}
            choices={views}
            disabled={definition.groups.length < 2}
            onChange={(match) => setDefinition({ ...definition, op: opOf(match) })}
          />
          <span className="text-body-medium text-text-secondary">of the groups</span>
        </div>
        <div className="flex flex-col">
          {definition.groups.map((group, i) => (
            <div key={group.key} className="flex flex-col">
              {i > 0 ? <CardJoin match={matchOf(definition.op)} /> : null}
              <GroupEditor
                workspace={workspace}
                index={i}
                group={group}
                only={definition.groups.length === 1}
                warnings={view?.warnings ?? []}
                onChange={(next) => setGroup(i, next)}
                onRemove={() => setGroup(i, null)}
              />
            </div>
          ))}
        </div>
        {definition.groups.length < 5 ? (
          <div>
            <Button
              variant="secondary"
              leadingIcon={RiAddLine}
              onClick={() =>
                setDefinition({
                  ...definition,
                  groups: [
                    ...definition.groups,
                    { key: crypto.randomUUID(), op: "and", conditions: [keyedCondition()] },
                  ],
                })
              }
            >
              Add group
            </Button>
          </div>
        ) : null}
        {refusal ? <p className="text-body-2-regular text-text-error-primary">{refusal}</p> : null}
      </div>
      <EditorFooter
        editing={view !== null}
        named={name.trim() !== ""}
        filtering={hasConditions(unkeyed(definition))}
        saving={save.isPending}
        onDelete={() => remove.mutate()}
        onCancel={onClose}
        onSave={() => save.mutate()}
        onApply={
          view === null && onApply
            ? () => {
                onApply(unkeyed(definition));
                onClose();
              }
            : undefined
        }
      />
    </>
  );
}
