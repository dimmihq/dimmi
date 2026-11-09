/*
 * What the people list keeps outside its components: the customer the
 * address opens, how table sorting maps to the server's sorts, and the
 * columns' widths.
 */
import type { SortingState } from "@tanstack/react-table";

import type { PeopleFilters } from "@/lib/api/customers";

/*
 * audienceSearch reads the audience the address opens in the drawer.
 */
export function audienceSearch(search: Record<string, unknown>): { audience?: string } {
  return typeof search.audience === "string" && search.audience !== "" ? { audience: search.audience } : {};
}

/*
 * companySearch reads the company the address opens in the drawer.
 */
export function companySearch(search: Record<string, unknown>): { company?: string } {
  return typeof search.company === "string" && search.company !== "" ? { company: search.company } : {};
}

/*
 * peopleSearch reads the customer the address opens in the drawer.
 */
export function peopleSearch(search: Record<string, unknown>): { contact?: string } {
  return typeof search.contact === "string" && search.contact !== "" ? { contact: search.contact } : {};
}

/*
 * sortOf maps a sortable column to the server's sort, and back.
 */
export const sortOf: Record<string, NonNullable<PeopleFilters["sort"]>> = {
  name: "name",
  last_seen: "last_seen_at",
  created: "-created_at",
};

export function sortingOf(sort: PeopleFilters["sort"]): SortingState {
  const id = Object.keys(sortOf).find((key) => sortOf[key] === (sort ?? "last_seen_at")) ?? "last_seen";

  return [{ id, desc: id !== "name" }];
}

export function sortFrom(sorting: SortingState): PeopleFilters["sort"] {
  return sortOf[sorting[0]?.id ?? "last_seen"] ?? "last_seen_at";
}

export const peopleWidths: Record<string, string> = {
  name: "w-[220px]",
  email: "w-[260px]",
  company: "w-[200px]",
  kind: "w-[110px]",
  tags: "w-[220px]",
  phone: "w-[160px]",
  user_id: "w-[160px]",
  last_seen: "w-[150px]",
  created: "w-[150px]",
  actions: "w-[64px]",
};
