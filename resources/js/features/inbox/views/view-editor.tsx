/*
 * The editor of a saved view, in a drawer: its name, who sees it, and its
 * conditions in groups. Within a group the conditions all apply, or any
 * of them; the groups join the same way. A new view can start from the
 * list on screen, keeping its status and sort. Saving opens the view.
 */
import { RiAddLine, RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Drawer, DrawerBody, DrawerFooter, DrawerHeader } from "@/components/base/drawer/drawer";
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { ApiError } from "@/lib/api/client";
import { createView, deleteView, type SavedView, updateView } from "@/lib/api/views";
import { callerQuery } from "@/lib/api/workspace";
import { realtimeKeys } from "@/lib/realtime/keys";

import { ConditionRow } from "./condition-row";
import { blankDefinition, type KeyedGroup, keyed, keyedCondition, unkeyed, warned } from "./fields";

const joins = { all: "and", any: "or" } as const;

const scopes = { personal: "personal", shared: "shared" } as const;

export interface ViewDraftStart {
  sort?: string;
  status?: string;
}

function Join({ label, value, onChange }: { label: string; value: "and" | "or"; onChange: (v: "and" | "or") => void }) {
  return (
    <SegmentedControl
      aria-label={label}
      variant="plain"
      selectedKeys={[value]}
      onSelectionChange={(selected) => onChange(selected.has("or") ? "or" : "and")}
      className="[&_button]:py-0.5 [&_button]:text-body-2-medium"
    >
      <SegmentedControlItem id={joins.all}>All</SegmentedControlItem>
      <SegmentedControlItem id={joins.any}>Any</SegmentedControlItem>
    </SegmentedControl>
  );
}

function GroupEditor({
  workspace,
  index,
  group,
  warnings,
  onChange,
  onRemove,
}: {
  workspace: string;
  index: number;
  group: KeyedGroup;
  warnings: SavedView["warnings"];
  onChange: (group: KeyedGroup) => void;
  onRemove: (() => void) | null;
}) {
  return (
    <fieldset className="flex flex-col gap-2 rounded-2xl border border-border-button-default p-3">
      <legend className="sr-only">Group {index + 1}</legend>
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-2 text-body-2-regular text-text-secondary">
          Match
          <Join label={`Group ${index + 1} matches`} value={group.op} onChange={(op) => onChange({ ...group, op })} />
          of these
        </span>
        {onRemove ? (
          <Button size="xs" variant="ghost" onClick={onRemove}>
            Remove group
          </Button>
        ) : null}
      </div>
      {group.conditions.map((condition, i) => (
        <ConditionRow
          key={condition.key}
          workspace={workspace}
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
      <Button
        size="xs"
        variant="secondary"
        leadingIcon={RiAddLine}
        onClick={() => onChange({ ...group, conditions: [...group.conditions, keyedCondition()] })}
        className="self-start"
      >
        Add condition
      </Button>
    </fieldset>
  );
}

export function ViewEditor({
  workspace,
  view,
  start,
  isOpen,
  onOpenChange,
}: {
  workspace: string;
  view: SavedView | null;
  start?: ViewDraftStart;
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const canShare = caller?.member?.permissions.includes("views:manage_shared") ?? false;
  const [name, setName] = useState(view?.name ?? "");
  const [scope, setScope] = useState<SavedView["scope"]>(view?.scope ?? "personal");
  const [definition, setDefinition] = useState(() => keyed(view?.definition ?? blankDefinition()));
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
      onOpenChange(false);
      void navigate({ to: "/d/inbox/$view", params: { view: saved.id } });
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteView(workspace, view?.id ?? ""),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: realtimeKeys.views(workspace) });
      onOpenChange(false);
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

  return (
    <Drawer isOpen={isOpen} onOpenChange={onOpenChange} size="lg">
      <DrawerHeader title={view ? "Edit view" : "New view"} description="Conversations that match, listed live." />
      <DrawerBody className="flex flex-col gap-4">
        <Input label="Name" value={name} onChange={setName} maxLength={60} />
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
        {definition.groups.length > 1 ? (
          <span className="flex items-center gap-2 text-body-2-regular text-text-secondary">
            Match
            <Join label="Groups match" value={definition.op} onChange={(op) => setDefinition({ ...definition, op })} />
            groups
          </span>
        ) : null}
        {definition.groups.map((group, i) => (
          <GroupEditor
            key={group.key}
            workspace={workspace}
            index={i}
            group={group}
            warnings={view?.warnings ?? []}
            onChange={(next) => setGroup(i, next)}
            onRemove={definition.groups.length > 1 ? () => setGroup(i, null) : null}
          />
        ))}
        {definition.groups.length < 5 ? (
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiAddLine}
            onClick={() =>
              setDefinition({
                ...definition,
                groups: [...definition.groups, { key: crypto.randomUUID(), op: "and", conditions: [keyedCondition()] }],
              })
            }
            className="self-start"
          >
            Add group
          </Button>
        ) : null}
        {refusal ? <p className="text-body-2-regular text-text-error-primary">{refusal}</p> : null}
      </DrawerBody>
      <DrawerFooter>
        {view ? (
          <Button variant="danger-secondary" leadingIcon={RiDeleteBinLine} onClick={() => remove.mutate()}>
            Delete
          </Button>
        ) : null}
        <Button onClick={() => save.mutate()} loading={save.isPending} disabled={name.trim() === ""}>
          Save view
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}
