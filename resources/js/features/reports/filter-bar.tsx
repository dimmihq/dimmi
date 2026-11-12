/*
 * The bar above a report: the range, compare and business hours, and a
 * picker for each filter. Every choice is written to the address, so a
 * view can be shared as a link. A live report has no range.
 */
import { getLocalTimeZone, today } from "@internationalized/date";
import { RiArrowDownSLine, RiSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { DateRangePicker, type DateRangePreset } from "@/components/base/date-picker/date-range-picker";
import { InputBase } from "@/components/base/input/input";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { Switch } from "@/components/base/switch/switch";
import { inboxesQuery } from "@/lib/api/inboxes";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { channelOptions, type Option, priorityOptions } from "./options";
import { calendarRange, presetDays, type RangePreset, type ReportSearch, rangePresets } from "./search";

const aiOptions: Option[] = [
  { id: "involved", label: "AI involved" },
  { id: "not_involved", label: "No AI" },
];

const chip =
  "inline-flex h-9 max-w-64 shrink-0 items-center gap-1 rounded-full border border-border-button-default bg-background-secondary-default ps-3 pe-2 text-body-medium outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring";

function split(value: string | undefined): string[] {
  return value ? value.split(",").filter(Boolean) : [];
}

/*
 * Picker picks any number of values of one filter, from the workspace's
 * own list, or one value when single is set.
 */
function Picker({
  label,
  options,
  value,
  onChange,
  single = false,
}: {
  label: string;
  options: Option[];
  value: string | undefined;
  onChange: (value: string | undefined) => void;
  single?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const picked = split(value);
  const q = text.trim().toLowerCase();
  const shown = options.filter((o) => o.label.toLowerCase().includes(q));
  const first = options.find((o) => o.id === picked[0])?.label ?? picked[0];
  const summary = picked.length === 0 ? label : picked.length === 1 ? first : `${label}: ${picked.length}`;
  const toggle = (id: string, on: boolean) => {
    if (single) {
      onChange(on ? id : undefined);
      setOpen(false);
      return;
    }
    const next = on ? [...picked, id] : picked.filter((p) => p !== id);
    onChange(next.length > 0 ? next.join(",") : undefined);
  };

  return (
    <PopoverTrigger
      isOpen={open}
      onOpenChange={(next) => {
        setOpen(next);
        setText("");
      }}
    >
      <button
        type="button"
        aria-label={picked.length === 0 ? `Filter by ${label.toLowerCase()}` : `${label}: ${summary}`}
        className={cx(chip, picked.length > 0 ? "text-text-primary" : "text-text-secondary")}
      >
        <span className="truncate">{summary}</span>
        <RiArrowDownSLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
      </button>
      <Popover aria-label={label} placement="bottom start" className="w-64">
        <div className="flex flex-col gap-2">
          {options.length > 8 ? (
            <InputBase
              aria-label={`Search ${label.toLowerCase()}`}
              placeholder="Search"
              leadingIcon={RiSearchLine}
              value={text}
              onChange={(event) => setText(event.target.value)}
              className="text-body-medium"
            />
          ) : null}
          <div className="-mx-1 flex max-h-72 flex-col gap-0.5 overflow-y-auto px-1">
            {shown.map((option) => (
              <Checkbox
                key={option.id}
                isSelected={picked.includes(option.id)}
                onChange={(on) => toggle(option.id, on)}
                className="rounded-lg px-2 py-1.5 hover:bg-background-secondary-hover"
              >
                <span className="truncate">{option.label}</span>
              </Checkbox>
            ))}
            {shown.length === 0 ? (
              <p className="px-2 py-3 text-body-2-regular text-text-tertiary">
                {options.length === 0 ? "Nothing to pick yet." : "Nothing matches."}
              </p>
            ) : null}
          </div>
          {picked.length > 0 ? (
            <LinkButton
              variant="secondary"
              className="self-start px-2"
              onClick={() => {
                onChange(undefined);
                setOpen(false);
              }}
            >
              Clear
            </LinkButton>
          ) : null}
        </div>
      </Popover>
    </PopoverTrigger>
  );
}

function presetList(): DateRangePreset[] {
  return rangePresets.flatMap((p) => {
    const days = presetDays(p.id);
    const range = calendarRange(days.from, days.to);
    return range ? [{ id: p.id, label: p.label, range }] : [];
  });
}

export function FilterBar({
  workspace,
  search,
  live,
  onChange,
}: {
  workspace: string;
  search: ReportSearch;
  live: boolean;
  onChange: (changes: Partial<ReportSearch>) => void;
}) {
  const enabled = workspace !== "";
  const inboxes = useQuery({ ...inboxesQuery(workspace), enabled });
  const teams = useQuery({ ...teamsQuery(workspace), enabled });
  const members = useQuery({ ...membersQuery(workspace), enabled });
  const tags = useQuery({ ...tagsQuery(workspace), enabled });
  const preset: RangePreset | null = search.range ?? (search.from || search.to ? null : "30d");
  const days = preset ? presetDays(preset) : null;
  const value = days
    ? calendarRange(days.from, days.to)
    : calendarRange(search.from ?? search.to ?? "", search.to ?? search.from ?? "");
  const filtered = Boolean(
    search.inbox || search.team || search.member || search.channel || search.tag || search.priority || search.ai,
  );

  return (
    <div className="flex flex-wrap items-center gap-2">
      {live ? null : (
        <DateRangePicker
          aria-label="Date range"
          className="py-1.5"
          value={value}
          preset={preset}
          presets={presetList()}
          maxValue={today(getLocalTimeZone())}
          onChange={(next, picked) => {
            const known = rangePresets.find((p) => p.id === picked);
            if (known) {
              onChange({ range: known.id, from: undefined, to: undefined });
            } else if (next) {
              onChange({ range: undefined, from: next.start.toString(), to: next.end.toString() });
            } else {
              onChange({ range: undefined, from: undefined, to: undefined });
            }
          }}
        />
      )}
      {live ? null : (
        <Switch
          size="sm"
          isSelected={search.compare === true}
          onChange={(on) => onChange({ compare: on || undefined })}
        >
          Compare
        </Switch>
      )}
      <Switch
        size="sm"
        isSelected={search.business_hours === true}
        onChange={(on) => onChange({ business_hours: on || undefined })}
      >
        Business hours
      </Switch>
      <span aria-hidden className="hidden h-5 w-px bg-separator-border sm:block" />
      <Picker
        label="Inbox"
        options={(inboxes.data ?? []).map((i) => ({ id: i.id, label: i.name }))}
        value={search.inbox}
        onChange={(inbox) => onChange({ inbox })}
      />
      <Picker
        label="Team"
        options={(teams.data ?? []).map((t) => ({ id: t.id, label: t.name }))}
        value={search.team}
        onChange={(team) => onChange({ team })}
      />
      <Picker
        label="Teammate"
        options={(members.data ?? [])
          .filter((m) => m.status === "active")
          .map((m) => ({ id: m.id, label: m.user.name || m.user.email }))}
        value={search.member}
        onChange={(member) => onChange({ member })}
      />
      <Picker
        label="Channel"
        options={channelOptions}
        value={search.channel}
        onChange={(channel) => onChange({ channel })}
      />
      <Picker
        label="Tag"
        options={(tags.data ?? []).map((t) => ({ id: t.id, label: t.name }))}
        value={search.tag}
        onChange={(tag) => onChange({ tag })}
      />
      <Picker
        label="Priority"
        options={priorityOptions}
        value={search.priority}
        onChange={(priority) => onChange({ priority })}
      />
      <Picker
        label="AI"
        single
        options={aiOptions}
        value={search.ai}
        onChange={(ai) => onChange({ ai: ai === "involved" || ai === "not_involved" ? ai : undefined })}
      />
      {filtered ? (
        <LinkButton
          variant="secondary"
          onClick={() =>
            onChange({
              inbox: undefined,
              team: undefined,
              member: undefined,
              channel: undefined,
              tag: undefined,
              priority: undefined,
              ai: undefined,
            })
          }
        >
          Clear filters
        </LinkButton>
      ) : null}
    </div>
  );
}
