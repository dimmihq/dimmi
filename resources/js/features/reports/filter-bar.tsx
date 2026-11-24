/*
 * The bar above a report. The range leads, then one Filter menu that
 * holds every filter: it lists them, and picking one slides in its
 * values to tick. Each filter in use shows as a chip that opens the same
 * values to change it, with a cross to remove it. Compare and office
 * hours sit at the far end. Every choice is written to the address, so
 * a view can be shared as a link. A live report has no range.
 */
import { getLocalTimeZone, today } from "@internationalized/date";
import {
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiAtLine,
  RiCloseLine,
  RiFilter3Line,
  RiFlag2Line,
  RiInbox2Line,
  RiPriceTag3Line,
  RiRobot2Line,
  RiSearchLine,
  RiSparkling2Line,
  RiTeamLine,
  RiUserLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { type ComponentType, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { DateRangePicker, type DateRangePreset } from "@/components/base/date-picker/date-range-picker";
import { InputBase } from "@/components/base/input/input";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { Switch } from "@/components/base/switch/switch";
import { agentsQuery } from "@/lib/api/agents";
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

type FilterKey = "inbox" | "team" | "member" | "channel" | "tag" | "priority" | "ai" | "agent";

/*
 * FilterDef is one filter: what it is called, its mark, the values it
 * offers, and whether it takes one value only.
 */
interface FilterDef {
  key: FilterKey;
  label: string;
  icon: ComponentType<{ className?: string }>;
  options: Option[];
  single?: boolean;
}

const row = cx(
  "flex min-h-9 w-full items-center gap-2.5 rounded-xl px-2 text-start text-body-medium text-text-primary",
  "outline-none hover:bg-background-primary-hover focus-visible:bg-background-primary-hover",
);

const slide = "motion-safe:animate-[filter-panel-in_180ms_ease-out]";

function split(value: string | undefined): string[] {
  return value ? value.split(",").filter(Boolean) : [];
}

/*
 * summaryOf says what a filter is set to: the one value by name, or how
 * many.
 */
function summaryOf(def: FilterDef, picked: string[]): string {
  if (picked.length === 1) {
    return def.options.find((o) => o.id === picked[0])?.label ?? "1 picked";
  }

  return `${picked.length} picked`;
}

/*
 * Values lists a filter's values to tick, searchable when there are
 * many. Each tick applies at once; a filter that takes one value closes
 * on the pick.
 */
function Values({
  def,
  value,
  onChange,
  onDone,
  onBack,
}: {
  def: FilterDef;
  value: string | undefined;
  onChange: (value: string | undefined) => void;
  onDone: () => void;
  onBack?: () => void;
}) {
  const [text, setText] = useState("");
  const picked = split(value);
  const q = text.trim().toLowerCase();
  const shown = def.options.filter((o) => o.label.toLowerCase().includes(q));
  const toggle = (id: string, on: boolean) => {
    if (def.single) {
      onChange(on ? id : undefined);
      onDone();
      return;
    }
    const next = on ? [...picked, id] : picked.filter((p) => p !== id);
    onChange(next.length > 0 ? next.join(",") : undefined);
  };

  return (
    <div className={cx("flex flex-col gap-2", slide)}>
      <div className={cx("flex min-h-8 items-center", onBack ? "gap-0.5 ps-0.5" : "ps-2")}>
        {onBack ? (
          <button
            type="button"
            aria-label="Back"
            onClick={onBack}
            className="grid size-7 shrink-0 place-items-center rounded-lg text-foreground-icon-secondary outline-none hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiArrowLeftSLine aria-hidden className="size-4 rtl:-scale-x-100" />
          </button>
        ) : null}
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{def.label}</span>
        {picked.length > 0 ? (
          <LinkButton size="xs" variant="secondary" className="me-2" onClick={() => onChange(undefined)}>
            Clear
          </LinkButton>
        ) : null}
      </div>
      {def.options.length > 8 ? (
        <InputBase
          aria-label={`Search ${def.label.toLowerCase()}`}
          placeholder="Search"
          leadingIcon={RiSearchLine}
          autoFocus
          value={text}
          onChange={(event) => setText(event.target.value)}
          className="text-body-medium"
        />
      ) : null}
      <div className="-mx-1 flex max-h-72 flex-col overflow-y-auto px-1">
        {shown.map((option) => (
          <Checkbox
            key={option.id}
            size="sm"
            isSelected={picked.includes(option.id)}
            onChange={(on) => toggle(option.id, on)}
            className="min-h-9 w-full cursor-pointer items-center rounded-xl px-2 transition-colors duration-150 hover:bg-background-primary-hover [&>span:first-child]:mt-0"
          >
            <span className="truncate">{option.label}</span>
          </Checkbox>
        ))}
        {shown.length === 0 ? (
          <p className="px-2 py-3 text-body-2-regular text-text-tertiary">
            {def.options.length === 0 ? "Nothing to pick yet." : "Nothing matches."}
          </p>
        ) : null}
      </div>
    </div>
  );
}

type Change = (changes: Partial<ReportSearch>) => void;

/*
 * setOf writes one filter's value to the address. AI takes only its two
 * values.
 */
function setOf(key: FilterKey, value: string | undefined): Partial<ReportSearch> {
  if (key === "ai") {
    return { ai: value === "involved" || value === "not_involved" ? value : undefined };
  }

  return { [key]: value };
}

/*
 * FilterMenu is the one way in to every filter: the list of them, each
 * saying what it is set to, and the values of the one picked.
 */
function FilterMenu({ defs, search, onChange }: { defs: FilterDef[]; search: ReportSearch; onChange: Change }) {
  const [open, setOpen] = useState(false);
  const [key, setKey] = useState<FilterKey | null>(null);
  const def = defs.find((d) => d.key === key);
  const active = defs.filter((d) => search[d.key]).length;

  return (
    <PopoverTrigger
      isOpen={open}
      onOpenChange={(next) => {
        setOpen(next);
        setKey(null);
      }}
    >
      <Button variant="secondary" size="small" leadingIcon={RiFilter3Line} className="shrink-0">
        Filter
        {active > 0 ? (
          <span className="ms-1 grid h-4 min-w-4 place-items-center rounded-full bg-button-primary px-1 text-caption-2-medium text-text-white tabular-nums">
            {active}
          </span>
        ) : null}
      </Button>
      <Popover aria-label="Filter the report" placement="bottom start" className="w-64 overflow-hidden">
        {def ? (
          <div key={def.key}>
            <Values
              def={def}
              value={search[def.key]}
              onChange={(value) => onChange(setOf(def.key, value))}
              onDone={() => setOpen(false)}
              onBack={() => setKey(null)}
            />
          </div>
        ) : (
          <div className={cx("flex flex-col", slide)}>
            <p className="px-2 py-1.5 text-caption-1-medium text-text-tertiary">Filter by</p>
            {defs.map((d) => {
              const picked = split(search[d.key]);

              return (
                <button key={d.key} type="button" onClick={() => setKey(d.key)} className={row}>
                  <d.icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                  <span className="min-w-0 flex-1 truncate">{d.label}</span>
                  {picked.length > 0 ? (
                    <span className="max-w-24 shrink-0 truncate text-body-2-regular text-text-tertiary">
                      {summaryOf(d, picked)}
                    </span>
                  ) : null}
                  <RiArrowRightSLine
                    aria-hidden
                    className="size-4 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                  />
                </button>
              );
            })}
          </div>
        )}
      </Popover>
    </PopoverTrigger>
  );
}

/*
 * FilterChip is one filter in use: it opens its values to change it, and
 * its cross removes it.
 */
function FilterChip({ def, value, onChange }: { def: FilterDef; value: string; onChange: Change }) {
  const [open, setOpen] = useState(false);
  const text = `${def.label}: ${summaryOf(def, split(value))}`;

  return (
    <span className="inline-flex h-8 max-w-72 items-center rounded-full border border-border-button-default bg-background-secondary-default ps-1">
      <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
        <button
          type="button"
          aria-label={`Change filter: ${text}`}
          className="inline-flex h-full min-w-0 items-center gap-1.5 rounded-full ps-2 pe-1 text-body-2-medium text-text-primary outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <def.icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <span className="truncate">{text}</span>
        </button>
        <Popover aria-label={def.label} placement="bottom start" className="w-64">
          <Values
            def={def}
            value={value}
            onChange={(next) => onChange(setOf(def.key, next))}
            onDone={() => setOpen(false)}
          />
        </Popover>
      </PopoverTrigger>
      <button
        type="button"
        aria-label={`Remove filter: ${text}`}
        onClick={() => onChange(setOf(def.key, undefined))}
        className="me-1 grid size-6 shrink-0 place-items-center rounded-full text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCloseLine aria-hidden className="size-4" />
      </button>
    </span>
  );
}

function presetList(): DateRangePreset[] {
  return rangePresets.flatMap((p) => {
    const days = presetDays(p.id);
    const range = calendarRange(days.from, days.to);
    return range ? [{ id: p.id, label: p.label, range }] : [];
  });
}

/*
 * useFilterDefs builds the filters from the workspace's own lists.
 */
function useFilterDefs(workspace: string): FilterDef[] {
  const enabled = workspace !== "";
  const inboxes = useQuery({ ...inboxesQuery(workspace), enabled });
  const teams = useQuery({ ...teamsQuery(workspace), enabled });
  const members = useQuery({ ...membersQuery(workspace), enabled });
  const tags = useQuery({ ...tagsQuery(workspace), enabled });
  const agents = useQuery({ ...agentsQuery(workspace), enabled });

  return [
    {
      key: "inbox",
      label: "Inbox",
      icon: RiInbox2Line,
      options: (inboxes.data ?? []).map((i) => ({ id: i.id, label: i.name })),
    },
    {
      key: "team",
      label: "Team",
      icon: RiTeamLine,
      options: (teams.data ?? []).map((t) => ({ id: t.id, label: t.name })),
    },
    {
      key: "member",
      label: "Teammate",
      icon: RiUserLine,
      options: (members.data ?? [])
        .filter((m) => m.status === "active")
        .map((m) => ({ id: m.id, label: m.user.name || m.user.email })),
    },
    { key: "channel", label: "Channel", icon: RiAtLine, options: channelOptions },
    {
      key: "tag",
      label: "Tag",
      icon: RiPriceTag3Line,
      options: (tags.data ?? []).map((t) => ({ id: t.id, label: t.name })),
    },
    { key: "priority", label: "Priority", icon: RiFlag2Line, options: priorityOptions },
    { key: "ai", label: "AI", icon: RiSparkling2Line, options: aiOptions, single: true },
    {
      key: "agent",
      label: "AI agent",
      icon: RiRobot2Line,
      options: (agents.data ?? []).map((a) => ({ id: a.id, label: a.name })),
    },
  ];
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
  onChange: Change;
}) {
  const defs = useFilterDefs(workspace);
  const preset: RangePreset | null = search.range ?? (search.from || search.to ? null : "30d");
  const days = preset ? presetDays(preset) : null;
  const value = days
    ? calendarRange(days.from, days.to)
    : calendarRange(search.from ?? search.to ?? "", search.to ?? search.from ?? "");
  const inUse = defs.filter((d) => search[d.key]);

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-2">
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
      <FilterMenu defs={defs} search={search} onChange={onChange} />
      {inUse.map((d) => (
        <FilterChip key={d.key} def={d} value={search[d.key] ?? ""} onChange={onChange} />
      ))}
      {inUse.length > 1 ? (
        <LinkButton
          variant="secondary"
          size="xs"
          className="ms-1"
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
          Clear all
        </LinkButton>
      ) : null}
      <div className="ms-auto flex shrink-0 items-center gap-4">
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
          Office hours
        </Switch>
      </div>
    </div>
  );
}
