/*
 * One condition of a saved view in the editor: the field, how it
 * compares, and the value, whose control follows the kind of value the
 * operator takes. A condition naming something that was deleted is
 * marked, so the person can fix or remove it.
 */
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { Key } from "react-aria-components";

import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { RuleRow } from "@/features/audiences/rule-layout";
import { conversationPriorities, inboxesQuery } from "@/lib/api/inbox";
import { teamsQuery } from "@/lib/api/teams";
import type { ViewCondition } from "@/lib/api/views";
import { membersQuery } from "@/lib/api/workspace";

import { priorityLabel } from "../priority/labels";
import { defaultValue, durations, fieldSpecs, opFor, type ValueKind } from "./fields";

interface Option {
  id: string;
  label: string;
}

function Choice({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string | undefined;
  options: Option[];
  onChange: (value: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      selectedKey={value ?? null}
      onSelectionChange={(key: Key | null) => {
        if (key !== null) {
          onChange(String(key));
        }
      }}
      className="w-full min-w-0"
    >
      {options.map((o) => (
        <SelectItem key={o.id} id={o.id} textValue={o.label}>
          {o.label}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * useOptions returns the choices of a kind of value, read from the
 * workspace where they live there.
 */
function useOptions(workspace: string, kind: ValueKind | undefined): Option[] {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: kind === "member" });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: kind === "team" });
  const { data: inboxes = [] } = useQuery({ ...inboxesQuery(workspace), enabled: kind === "inbox" });

  switch (kind) {
    case "statuses":
      return [
        { id: "open", label: "Open" },
        { id: "snoozed", label: "Snoozed" },
        { id: "closed", label: "Done" },
      ];
    case "level":
      return conversationPriorities.map((p) => ({ id: p, label: priorityLabel[p] }));
    case "member":
      return [{ id: "me", label: "Me" }, ...members.map((m) => ({ id: m.id, label: m.user.name }))];
    case "team":
      return teams.map((t) => ({ id: t.id, label: t.name }));
    case "inbox":
      return inboxes.map((i) => ({ id: i.id, label: i.name }));
    case "channels":
      return [
        { id: "email", label: "Email" },
        { id: "widget", label: "Messenger" },
        { id: "api", label: "API" },
      ];
    case "flag":
      return [
        { id: "true", label: "Yes" },
        { id: "false", label: "No" },
      ];
    case "duration":
      return durations.map((d) => ({ id: d.value, label: d.label }));
    default:
      return [];
  }
}

/*
 * listKinds are the kinds whose value is a list with one entry.
 */
const listKinds: (ValueKind | undefined)[] = ["statuses", "member", "team", "inbox", "channels"];

function ValueControl({
  workspace,
  kind,
  value,
  onChange,
}: {
  workspace: string;
  kind: ValueKind | undefined;
  value: unknown;
  onChange: (value: unknown) => void;
}): ReactNode {
  const options = useOptions(workspace, kind);
  if (kind === undefined) {
    return null;
  }
  if (kind === "text") {
    return <Input aria-label="Value" value={String(value ?? "")} onChange={onChange} className="w-full" />;
  }
  const listed = listKinds.includes(kind);
  const current = listed ? (value as string[] | undefined)?.[0] : String(value ?? "");

  return (
    <Choice
      label="Value"
      value={current}
      options={options}
      onChange={(picked) => onChange(listed ? [picked] : kind === "flag" ? picked === "true" : picked)}
    />
  );
}

export function ConditionRow({
  workspace,
  join,
  condition,
  flagged,
  onChange,
  onRemove,
}: {
  workspace: string;
  join: string;
  condition: ViewCondition;
  flagged: boolean;
  onChange: (condition: ViewCondition) => void;
  onRemove: () => void;
}) {
  const spec = fieldSpecs.find((f) => f.field === condition.field);
  const op = opFor(condition);

  return (
    <RuleRow
      join={join}
      flagged={flagged}
      field={
        <Choice
          label="Field"
          value={condition.field}
          options={fieldSpecs.map((f) => ({ id: f.field, label: f.label }))}
          onChange={(field) => {
            const first = fieldSpecs.find((f) => f.field === field)?.ops[0];
            onChange({ field, op: first?.op ?? "eq", value: defaultValue(first?.kind) });
          }}
        />
      }
      op={
        <Choice
          label="Comparison"
          value={op?.op}
          options={(spec?.ops ?? []).map((o) => ({ id: o.op, label: o.label }))}
          onChange={(picked) => {
            const next = spec?.ops.find((o) => o.op === picked);
            onChange({ field: condition.field, op: picked, value: defaultValue(next?.kind) });
          }}
        />
      }
      value={
        op?.kind === undefined ? (
          <Input aria-label="Value" isDisabled placeholder="No value needed" className="w-full" />
        ) : (
          <ValueControl
            workspace={workspace}
            kind={op.kind}
            value={condition.value}
            onChange={(value) => onChange({ ...condition, value })}
          />
        )
      }
      below={
        flagged ? (
          <span className="text-body-2-regular text-text-error-primary">Names something that was deleted.</span>
        ) : null
      }
      onRemove={onRemove}
    />
  );
}
