/*
 * The filters above the people list: kind, company, tag, when they were
 * last seen or added, one custom attribute, and the order. Each choice
 * narrows the list at once; "Any" clears it.
 */
import { RiEqualizerLine } from "@remixicon/react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { type Key, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverFooter, PopoverTrigger } from "@/components/base/popover/popover";
import { Select, SelectItem } from "@/components/base/select/select";
import { companiesQuery, type PeopleFilters, type Since } from "@/lib/api/customers";
import { tagsQuery } from "@/lib/api/tags";

import { useWorkspace } from "./names";

const any = "any";

const kinds = [
  { id: any, label: "Everyone" },
  { id: "user", label: "Users" },
  { id: "lead", label: "Leads" },
];

const sorts: Record<NonNullable<PeopleFilters["sort"]>, string> = {
  last_seen_at: "Last seen",
  "-created_at": "Newest",
  name: "Name",
};

const windows: { id: Since | typeof any; seen: string; created: string }[] = [
  { id: any, seen: "Seen any time", created: "Added any time" },
  { id: "day", seen: "Seen today", created: "Added today" },
  { id: "week", seen: "Seen this week", created: "Added this week" },
  { id: "month", seen: "Seen this month", created: "Added this month" },
  { id: "older", seen: "Not seen in a month", created: "Added over a month ago" },
];

function orUndefined<T extends string>(value: string): T | undefined {
  return value === any ? undefined : (value as T);
}

function AttributeFilter({
  attribute,
  onChange,
}: {
  attribute: PeopleFilters["attribute"];
  onChange: (next: PeopleFilters["attribute"]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [key, setKey] = useState(attribute?.key ?? "");
  const [value, setValue] = useState(attribute?.value ?? "");
  const apply = (next: PeopleFilters["attribute"]) => {
    onChange(next);
    setOpen(false);
  };

  return (
    <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
      <Button variant="secondary" size="small" leadingIcon={RiEqualizerLine}>
        {attribute ? `${attribute.key} is ${attribute.value}` : "Attribute"}
      </Button>
      <Popover aria-label="Filter by attribute" placement="bottom start" className="w-72">
        <form
          className="flex flex-col gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            apply({ key: key.trim(), value: value.trim() });
          }}
        >
          <Input label="Attribute" inputDir="ltr" placeholder="plan" autoFocus value={key} onChange={setKey} />
          <Input label="Value" placeholder="pro" value={value} onChange={setValue} />
          <PopoverFooter>
            <Button
              variant="ghost"
              size="small"
              onClick={() => {
                setKey("");
                setValue("");
                apply(undefined);
              }}
            >
              Clear
            </Button>
            <Button type="submit" size="small" disabled={key.trim() === "" || value.trim() === ""}>
              Apply
            </Button>
          </PopoverFooter>
        </form>
      </Popover>
    </PopoverTrigger>
  );
}

export function PeopleFiltersBar({
  filters,
  onChange,
}: {
  filters: PeopleFilters;
  onChange: (next: PeopleFilters) => void;
}) {
  const workspace = useWorkspace();
  const { data: tags = [] } = useQuery(tagsQuery(workspace));
  const companies = useInfiniteQuery({ ...companiesQuery(workspace, ""), enabled: workspace !== "" });
  const companyItems = [
    { id: any, name: "Any company" },
    ...(companies.data?.pages.flatMap((page) => page.items) ?? []),
  ];
  const pick = (apply: (value: string) => PeopleFilters) => (key: Key | null) => {
    if (key !== null) {
      onChange(apply(String(key)));
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select
        aria-label="Kind"
        size="sm"
        className="w-32"
        selectedKey={filters.kind ?? any}
        onSelectionChange={pick((kind) => ({ ...filters, kind: orUndefined(kind) }))}
      >
        {kinds.map((kind) => (
          <SelectItem key={kind.id} id={kind.id}>
            {kind.label}
          </SelectItem>
        ))}
      </Select>
      <Select
        aria-label="Company"
        size="sm"
        className="w-40"
        selectedKey={filters.company ?? any}
        onSelectionChange={pick((company) => ({ ...filters, company: orUndefined(company) }))}
      >
        {companyItems.map((company) => (
          <SelectItem key={company.id} id={company.id} textValue={company.name}>
            {company.name}
          </SelectItem>
        ))}
      </Select>
      <Select
        aria-label="Tag"
        size="sm"
        className="w-40"
        selectedKey={filters.tag ?? any}
        onSelectionChange={pick((tag) => ({ ...filters, tag: orUndefined(tag) }))}
      >
        {[{ id: any, name: "Any tag" }, ...tags].map((tag) => (
          <SelectItem key={tag.id} id={tag.id} textValue={tag.name}>
            {tag.name}
          </SelectItem>
        ))}
      </Select>
      <Select
        aria-label="Last seen"
        size="sm"
        className="w-48"
        selectedKey={filters.seen ?? any}
        onSelectionChange={pick((seen) => ({ ...filters, seen: orUndefined(seen) }))}
      >
        {windows.map((window) => (
          <SelectItem key={window.id} id={window.id}>
            {window.seen}
          </SelectItem>
        ))}
      </Select>
      <Select
        aria-label="Added"
        size="sm"
        className="w-52"
        selectedKey={filters.created ?? any}
        onSelectionChange={pick((created) => ({ ...filters, created: orUndefined(created) }))}
      >
        {windows.map((window) => (
          <SelectItem key={window.id} id={window.id}>
            {window.created}
          </SelectItem>
        ))}
      </Select>
      <AttributeFilter attribute={filters.attribute} onChange={(attribute) => onChange({ ...filters, attribute })} />
      <Select
        aria-label="Sort"
        size="sm"
        className="w-36"
        selectedKey={filters.sort ?? "last_seen_at"}
        onSelectionChange={pick((sort) => ({ ...filters, sort: sort as PeopleFilters["sort"] }))}
      >
        {Object.entries(sorts).map(([id, label]) => (
          <SelectItem key={id} id={id}>
            {label}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}
