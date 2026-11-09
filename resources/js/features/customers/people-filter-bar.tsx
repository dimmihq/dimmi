/*
 * The people list's filters, as Intercom draws them: Add filter beside
 * the search, and a row of chips. Add filter lists the person's data and
 * the workspace's attributes, searchable, each with a caret. Picking a
 * field slides in its comparisons, such as "contains", and picking one
 * slides in its value with Apply, as Intercom does; a comparison needing
 * no value applies at once. Tags and companies take several picks at
 * once. A chip opens the same panels to change it, and its cross removes
 * it.
 */
import {
  RiAddLine,
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiCloseLine,
  RiGroupLine,
  RiSearchLine,
} from "@remixicon/react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverFooter, PopoverTrigger } from "@/components/base/popover/popover";
import { TagIcon } from "@/features/inbox/tags/tag-chip";
import { audiencesQuery } from "@/lib/api/audiences";
import { companiesQuery } from "@/lib/api/customers";
import { tagsQuery } from "@/lib/api/tags";
import { cx } from "@/utils/cx";

import { useWorkspace } from "./names";
import {
  chipText,
  comparisonOf,
  comparisonsOf,
  complete,
  type Filter,
  type FilterField,
  maxFilters,
  newFilter,
} from "./people-filter-fields";

const row = cx(
  "flex min-h-9 w-full items-center gap-2.5 rounded-xl px-2 text-start text-body-medium text-text-primary",
  "outline-none hover:bg-background-secondary-hover focus-visible:bg-background-secondary-hover",
);

function Search({ value, onChange, label }: { value: string; onChange: (next: string) => void; label: string }) {
  return (
    <Input
      aria-label={label}
      placeholder={label}
      leadingIcon={RiSearchLine}
      autoFocus
      value={value}
      onChange={onChange}
    />
  );
}

/*
 * Choice is one item of a list of picks, with what it reads as.
 */
interface Choice {
  id: string;
  label: string;
  leading?: ReactNode;
}

type Change = (next: Filter) => void;

/*
 * Picks lists choices to tick, several at once or one, searchable when
 * there are many.
 */
function Picks({
  choices,
  filter,
  many,
  onChange,
  search,
}: {
  choices: Choice[];
  filter: Filter;
  many: boolean;
  onChange: Change;
  search?: ReactNode;
}) {
  const toggle = (choice: Choice) => {
    const on = filter.values.includes(choice.id);
    const kept = many ? filter.values.filter((v) => v !== choice.id) : [];
    const values = on ? kept : [...kept, choice.id];
    const shown = values.map(
      (v) => choices.find((c) => c.id === v)?.label ?? filter.shown[filter.values.indexOf(v)] ?? v,
    );
    onChange({ ...filter, values, shown });
  };

  return (
    <div className="flex flex-col gap-2">
      {search}
      <div className="flex max-h-56 flex-col overflow-y-auto">
        {choices.map((choice) => (
          <Checkbox
            key={choice.id}
            size="sm"
            isSelected={filter.values.includes(choice.id)}
            onChange={() => toggle(choice)}
            className="min-h-9 rounded-xl px-2 hover:bg-background-secondary-hover"
          >
            <span className="flex min-w-0 items-center gap-2">
              {choice.leading}
              <span className="truncate">{choice.label}</span>
            </span>
          </Checkbox>
        ))}
        {choices.length === 0 ? (
          <p className="px-2 py-2 text-body-2-regular text-text-tertiary">Nothing matches.</p>
        ) : null}
      </div>
    </div>
  );
}

function TagPicks({ filter, onChange }: { filter: Filter; onChange: Change }) {
  const workspace = useWorkspace();
  const [text, setText] = useState("");
  const { data: tags = [] } = useQuery(tagsQuery(workspace));
  const q = text.trim().toLowerCase();
  const choices = tags
    .filter((t) => t.name.toLowerCase().includes(q))
    .map((t) => ({ id: t.id, label: t.name, leading: <TagIcon tag={t} /> }));

  return (
    <Picks
      choices={choices}
      filter={filter}
      many
      onChange={onChange}
      search={<Search label="Search tags" value={text} onChange={setText} />}
    />
  );
}

function CompanyPicks({ filter, onChange }: { filter: Filter; onChange: Change }) {
  const workspace = useWorkspace();
  const [text, setText] = useState("");
  const list = useInfiniteQuery({ ...companiesQuery(workspace, text.trim()), enabled: workspace !== "" });
  const choices = (list.data?.pages.flatMap((page) => page.items) ?? []).map((c) => ({ id: c.id, label: c.name }));

  return (
    <Picks
      choices={choices}
      filter={filter}
      many
      onChange={onChange}
      search={<Search label="Search companies" value={text} onChange={setText} />}
    />
  );
}

function AudiencePicks({ filter, onChange }: { filter: Filter; onChange: Change }) {
  const workspace = useWorkspace();
  const [text, setText] = useState("");
  const { data: audiences = [] } = useQuery(audiencesQuery(workspace));
  const q = text.trim().toLowerCase();
  const choices = audiences
    .filter((a) => a.name.toLowerCase().includes(q))
    .map((a) => ({
      id: a.id,
      label: a.name,
      leading: <RiGroupLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />,
    }));

  return (
    <Picks
      choices={choices}
      filter={filter}
      many
      onChange={onChange}
      search={<Search label="Search audiences" value={text} onChange={setText} />}
    />
  );
}

const kindLabels: Record<string, string> = { user: "Users", lead: "Leads" };

/*
 * Value is what the picked comparison asks for: typed text, a number, a
 * count of days, or picks.
 */
function Value({ field, filter, onChange }: { field: FilterField; filter: Filter; onChange: Change }) {
  const needs = comparisonOf(field, filter.op).needs;
  if (needs === "none") {
    return null;
  }
  if (field.kind === "tags") {
    return <TagPicks filter={filter} onChange={onChange} />;
  }
  if (field.kind === "company") {
    return <CompanyPicks filter={filter} onChange={onChange} />;
  }
  if (field.kind === "audience") {
    return <AudiencePicks filter={filter} onChange={onChange} />;
  }
  if (needs === "one") {
    const choices = (field.options ?? []).map((o) => ({ id: o, label: kindLabels[o] ?? o }));

    return <Picks choices={choices} filter={filter} many={false} onChange={onChange} />;
  }

  return (
    <div className="flex items-center gap-2">
      <Input
        aria-label={`${field.label} value`}
        type={needs === "text" ? "text" : "number"}
        inputDir={field.key === "email" || field.key === "user_id" || field.key === "phone" ? "ltr" : undefined}
        autoFocus
        className="flex-1"
        value={filter.value}
        onChange={(value) => onChange({ ...filter, value })}
        placeholder={needs === "days" ? "7" : "Value"}
      />
      {needs === "days" ? <span className="shrink-0 text-body-medium text-text-secondary">days ago</span> : null}
    </div>
  );
}

/*
 * Header tops a panel: a way back, when there is one, and what the panel
 * is about.
 */
function Header({ title, icon: Icon, onBack }: { title: string; icon?: FilterField["icon"]; onBack?: () => void }) {
  return (
    <div className="flex min-h-8 items-center gap-1.5">
      {onBack ? (
        <button
          type="button"
          aria-label="Back"
          onClick={onBack}
          className="grid size-7 place-items-center rounded-lg text-foreground-icon-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-4 rtl:-scale-x-100" />
        </button>
      ) : null}
      {Icon ? <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" /> : null}
      <span className="truncate text-body-medium text-text-primary">{title}</span>
    </div>
  );
}

const slide = "motion-safe:animate-[filter-panel-in_180ms_ease-out]";

/*
 * Editor walks one filter through its panels: the comparisons, each with
 * a caret when a value follows, then the value and Apply. A comparison
 * that needs no value applies as it is picked. A chip already complete
 * opens on its value.
 */
function Editor({
  field,
  start,
  onApply,
  onBack,
}: {
  field: FilterField;
  start: Filter;
  onApply: (filter: Filter) => void;
  onBack?: () => void;
}) {
  const [filter, setFilter] = useState(start);
  const [step, setStep] = useState<"compare" | "value">(complete(start, field) ? "value" : "compare");
  const current = comparisonOf(field, filter.op);

  if (step === "compare") {
    return (
      <div key="compare" className={cx("flex flex-col gap-1", slide)}>
        <Header title={field.label} icon={field.icon} onBack={onBack} />
        <div className="flex flex-col">
          {comparisonsOf(field).map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => {
                const next =
                  c.needs === current.needs
                    ? { ...filter, op: c.id }
                    : { ...newFilter(field), id: filter.id, op: c.id };
                if (c.needs === "none") {
                  onApply(next);
                  return;
                }
                setFilter(next);
                setStep("value");
              }}
              className={row}
            >
              <span className="min-w-0 flex-1 truncate">{c.label}</span>
              {c.needs === "none" ? null : (
                <RiArrowRightSLine
                  aria-hidden
                  className="size-4 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                />
              )}
            </button>
          ))}
        </div>
      </div>
    );
  }

  return (
    <form
      key="value"
      className={cx("flex flex-col gap-2", slide)}
      onSubmit={(event) => {
        event.preventDefault();
        if (complete(filter, field)) {
          onApply(filter);
        }
      }}
    >
      <Header title={`${field.label} ${current.label}`} icon={field.icon} onBack={() => setStep("compare")} />
      <Value field={field} filter={filter} onChange={setFilter} />
      <PopoverFooter>
        <Button type="submit" size="small" disabled={!complete(filter, field)}>
          Apply
        </Button>
      </PopoverFooter>
    </form>
  );
}

/*
 * AddFilter lists the fields, grouped and searchable, each sliding in its
 * panel when picked.
 */
export function AddFilter({
  fields,
  filters,
  onAdd,
}: {
  fields: FilterField[];
  filters: Filter[];
  onAdd: (filter: Filter) => void;
}) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [picked, setPicked] = useState<FilterField | null>(null);
  const q = text.trim().toLowerCase();
  const shown = fields.filter((f) => f.label.toLowerCase().includes(q));
  const groups = (["People data", "Your attributes"] as const).map((group) => ({
    group,
    items: shown.filter((f) => f.group === group),
  }));

  return (
    <PopoverTrigger
      isOpen={open}
      onOpenChange={(next) => {
        setOpen(next);
        setText("");
        setPicked(null);
      }}
    >
      <LinkButton
        variant="secondary"
        leadingIcon={RiAddLine}
        className="shrink-0"
        disabled={filters.length >= maxFilters}
      >
        Add filter
      </LinkButton>
      <Popover aria-label="Add filter" placement="bottom end" className="w-64 overflow-hidden">
        {picked ? (
          <div key={picked.key}>
            <Editor
              field={picked}
              start={newFilter(picked)}
              onBack={() => setPicked(null)}
              onApply={(filter) => {
                setOpen(false);
                setPicked(null);
                onAdd(filter);
              }}
            />
          </div>
        ) : (
          <div className={cx("flex flex-col gap-2", slide)}>
            <Search label="Search people data" value={text} onChange={setText} />
            <div className="-mx-1 flex max-h-80 flex-col overflow-y-auto px-1">
              {groups
                .filter((g) => g.items.length > 0)
                .map((g) => (
                  <div key={g.group} className="flex flex-col py-1">
                    <p className="px-2 py-1.5 text-caption-1-medium text-text-tertiary">{g.group}</p>
                    {g.items.map((field) => (
                      <button
                        key={field.key}
                        type="button"
                        title={field.description}
                        onClick={() => setPicked(field)}
                        className={row}
                      >
                        <field.icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                        <span className="min-w-0 flex-1 truncate">{field.label}</span>
                        <RiArrowRightSLine
                          aria-hidden
                          className="size-4 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                        />
                      </button>
                    ))}
                  </div>
                ))}
              {shown.length === 0 ? (
                <p className="px-2 py-3 text-body-2-regular text-text-tertiary">No field matches.</p>
              ) : null}
            </div>
          </div>
        )}
      </Popover>
    </PopoverTrigger>
  );
}

/*
 * Chip is one filter: it opens its panel to change it, and its cross
 * removes it.
 */
function Chip({
  filter,
  field,
  onChange,
  onRemove,
}: {
  filter: Filter;
  field: FilterField | undefined;
  onChange: Change;
  onRemove: () => void;
}) {
  const [open, setOpen] = useState(false);
  const Icon = field?.icon;
  const text = chipText(filter, field);

  return (
    <span className="inline-flex h-9 max-w-96 items-center rounded-full border border-border-button-default bg-background-secondary-default ps-1">
      <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
        <button
          type="button"
          aria-label={`Change filter: ${text}`}
          className="inline-flex h-full min-w-0 items-center gap-1.5 rounded-full ps-2 pe-1 text-body-medium text-text-primary outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          {Icon ? <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" /> : null}
          <span className="truncate">{text}</span>
        </button>
        <Popover aria-label={field?.label ?? "Filter"} placement="bottom start" className="w-64">
          {field ? (
            <Editor
              field={field}
              start={filter}
              onApply={(next) => {
                setOpen(false);
                onChange(next);
              }}
            />
          ) : null}
        </Popover>
      </PopoverTrigger>
      <button
        type="button"
        aria-label={`Remove filter: ${text}`}
        onClick={onRemove}
        className="me-1 grid size-7 shrink-0 place-items-center rounded-full text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCloseLine aria-hidden className="size-4" />
      </button>
    </span>
  );
}

/*
 * FilterBar is the row of chips, with a way to clear them all.
 */
export function FilterBar({
  fields,
  filters,
  onChange,
}: {
  fields: FilterField[];
  filters: Filter[];
  onChange: (next: Filter[]) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {filters.map((filter) => (
        <Chip
          key={filter.id}
          filter={filter}
          field={fields.find((f) => f.key === filter.field)}
          onChange={(next) => onChange(filters.map((f) => (f.id === next.id ? next : f)))}
          onRemove={() => onChange(filters.filter((f) => f.id !== filter.id))}
        />
      ))}
      <LinkButton variant="secondary" className="ms-1" onClick={() => onChange([])}>
        Clear filters
      </LinkButton>
    </div>
  );
}
