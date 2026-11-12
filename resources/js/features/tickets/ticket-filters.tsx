/*
 * The filters above a ticket view: type, state, group, assignee,
 * company, priority, linked issue status and one field's value. Each
 * picker is fed by the workspace's own data, the field picker lists the
 * fields of the types in view and, for choice fields, their options.
 * Filters in use show as chips under the button, each removable.
 */
import { RiCloseLine, RiFilter3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { type Key, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { Select, SelectItem } from "@/components/base/select/select";
import { companiesPageQuery } from "@/lib/api/customers";
import {
  type IssueStatus,
  stateGroups,
  type TicketFilter,
  type TicketPriority,
  ticketPriorities,
  ticketStatesQuery,
  ticketTypesQuery,
} from "@/lib/api/tickets";
import { membersQuery } from "@/lib/api/workspace";

import { groupLabel, issueStatusLabel, priorityLabel } from "./labels";

/*
 * Filters are what the person picked; the view's own filter comes first
 * and these narrow it.
 */
export type Filters = Pick<
  TicketFilter,
  "type_id" | "state_id" | "group" | "assignee_id" | "company_id" | "priority" | "issue_status" | "fields"
>;

const issueStatuses: IssueStatus[] = ["backlog", "unstarted", "started", "done", "canceled"];
const any = "any";

function pick(apply: (value: string | undefined) => void) {
  return (key: Key | null) => apply(key === null || key === any ? undefined : String(key));
}

interface Option {
  id: string;
  name: string;
}

function FilterSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string | undefined;
  options: Option[];
  onChange: (value: string | undefined) => void;
}) {
  return (
    <Select label={label} size="sm" selectedKey={value ?? any} onSelectionChange={pick(onChange)}>
      {[{ id: any, name: "Any" }, ...options].map((o) => (
        <SelectItem key={o.id} id={o.id} textValue={o.name}>
          {o.name}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * useFilterOptions reads what each picker offers, and names a picked
 * value for its chip.
 */
function useFilterOptions(workspace: string, typeIds: string[]) {
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const { data: states = [] } = useQuery(ticketStatesQuery(workspace));
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: workspace !== "" });
  const { data: companies } = useQuery({ ...companiesPageQuery(workspace, "", 1), enabled: workspace !== "" });
  const inView = types.filter((t) => !t.archived_at && (typeIds.length === 0 || typeIds.includes(t.id)));
  const fields = inView.flatMap((t) => t.fields.filter((f) => !f.archived_at && f.data_type !== "file"));

  return {
    types: types.filter((t) => !t.archived_at).map((t) => ({ id: t.id, name: t.name })),
    states: states.filter((s) => !s.archived_at).map((s) => ({ id: s.id, name: s.name })),
    groups: stateGroups.map((g) => ({ id: g, name: groupLabel(g) })),
    members: [
      { id: "none", name: "Unassigned" },
      ...members.filter((m) => m.status === "active").map((m) => ({ id: m.id, name: m.user.name })),
    ],
    companies: (companies?.items ?? []).map((c) => ({ id: c.id, name: c.name })),
    priorities: ticketPriorities.map((p) => ({ id: p, name: priorityLabel(p) })),
    issues: issueStatuses.map((s) => ({ id: s, name: issueStatusLabel(s) })),
    fields: [...new Map(fields.map((f) => [f.key, f])).values()],
  };
}

type Options = ReturnType<typeof useFilterOptions>;

/*
 * chipsOf names each filter in use.
 */
function chipsOf(filters: Filters, o: Options): { key: keyof Filters | `field:${string}`; text: string }[] {
  const name = (list: Option[], id?: string) => list.find((x) => x.id === id)?.name ?? id ?? "";
  const out: { key: keyof Filters | `field:${string}`; text: string }[] = [];
  const named: [keyof Filters, string, Option[]][] = [
    ["type_id", "Type", o.types],
    ["state_id", "State", o.states],
    ["group", "Group", o.groups],
    ["assignee_id", "Assignee", o.members],
    ["company_id", "Company", o.companies],
    ["priority", "Priority", o.priorities],
    ["issue_status", "Issue", o.issues],
  ];
  for (const [key, label, list] of named) {
    const value = filters[key];
    if (typeof value === "string" && value !== "") {
      out.push({ key, text: `${label}: ${name(list, value)}` });
    }
  }
  for (const [key, value] of Object.entries(filters.fields ?? {})) {
    const label = o.fields.find((f) => f.key === key)?.label ?? key;
    out.push({ key: `field:${key}`, text: `${label}: ${String(value)}` });
  }

  return out;
}

function FieldFilter({ options, onAdd }: { options: Options; onAdd: (key: string, value: string) => void }) {
  const [key, setKey] = useState<string | undefined>();
  const [value, setValue] = useState("");
  const field = options.fields.find((f) => f.key === key);
  const choice = field && (field.data_type === "single_choice" || field.data_type === "multiple_choice");

  return (
    <div className="flex flex-col gap-2 border-t border-separator-border pt-3">
      <FilterSelect
        label="Field"
        value={key}
        options={options.fields.map((f) => ({ id: f.key, name: f.label }))}
        onChange={(next) => {
          setKey(next);
          setValue("");
        }}
      />
      {field ? (
        <div className="flex items-end gap-2">
          {choice ? (
            <FilterSelect
              label="Is"
              value={value || undefined}
              options={field.options.map((x) => ({ id: x, name: x }))}
              onChange={(next) => setValue(next ?? "")}
            />
          ) : (
            <Input label="Is" size="small" value={value} onChange={setValue} className="flex-1" />
          )}
          <Button size="small" variant="secondary" disabled={value === ""} onClick={() => onAdd(field.key, value)}>
            Add
          </Button>
        </div>
      ) : null}
    </div>
  );
}

export function TicketFilters({
  workspace,
  filters,
  viewTypeId,
  onChange,
}: {
  workspace: string;
  filters: Filters;
  viewTypeId?: string;
  onChange: (filters: Filters) => void;
}) {
  const options = useFilterOptions(workspace, [viewTypeId ?? filters.type_id ?? ""].filter(Boolean));
  const set = (key: keyof Filters) => (value: string | undefined) => onChange({ ...filters, [key]: value });
  const chips = chipsOf(filters, options);
  const remove = (key: string) => {
    if (key.startsWith("field:")) {
      const { [key.slice(6)]: _, ...rest } = filters.fields ?? {};
      onChange({ ...filters, fields: rest });
      return;
    }
    onChange({ ...filters, [key]: undefined });
  };

  return (
    <div className="flex shrink-0 flex-col gap-2 px-[7px] pb-2">
      <div className="flex items-center gap-2">
        <PopoverTrigger>
          <Button size="small" variant="secondary" leadingIcon={RiFilter3Line}>
            Filter
          </Button>
          <Popover aria-label="Filter tickets" placement="bottom start" className="w-80" dialogClassName="p-3">
            <div className="flex max-h-[70vh] flex-col gap-3 overflow-y-auto">
              {viewTypeId ? null : (
                <FilterSelect label="Type" value={filters.type_id} options={options.types} onChange={set("type_id")} />
              )}
              <FilterSelect
                label="State"
                value={filters.state_id}
                options={options.states}
                onChange={set("state_id")}
              />
              <FilterSelect label="Group" value={filters.group} options={options.groups} onChange={set("group")} />
              <FilterSelect
                label="Assignee"
                value={filters.assignee_id}
                options={options.members}
                onChange={set("assignee_id")}
              />
              <FilterSelect
                label="Company"
                value={filters.company_id}
                options={options.companies}
                onChange={set("company_id")}
              />
              <FilterSelect
                label="Priority"
                value={filters.priority}
                options={options.priorities}
                onChange={(v) => onChange({ ...filters, priority: v as TicketPriority | undefined })}
              />
              <FilterSelect
                label="Linked issue"
                value={filters.issue_status}
                options={options.issues}
                onChange={(v) => onChange({ ...filters, issue_status: v as IssueStatus | undefined })}
              />
              <FieldFilter
                options={options}
                onAdd={(key, value) => onChange({ ...filters, fields: { ...filters.fields, [key]: value } })}
              />
            </div>
          </Popover>
        </PopoverTrigger>
        {chips.length > 0 ? (
          <Button size="small" variant="ghost" onClick={() => onChange({})}>
            Clear
          </Button>
        ) : null}
      </div>
      {chips.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {chips.map((chip) => (
            <button
              key={chip.key}
              type="button"
              onClick={() => remove(chip.key)}
              aria-label={`Remove ${chip.text}`}
              className="flex items-center gap-1 rounded-md bg-background-tertiary-default px-1.5 py-1 text-caption-1-medium text-text-secondary outline-none hover:bg-background-tertiary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              {chip.text}
              <RiCloseLine aria-hidden className="size-3.5" />
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
