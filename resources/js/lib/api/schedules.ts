/*
 * The workspace's office hours. Every workspace has a default schedule,
 * made on first read, which workflows use wherever they name none; each
 * schedule says whether it is open right now, so the browser never does
 * timezone arithmetic of its own. Changing them takes the
 * schedules:manage permission, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

/*
 * Weekly maps "mon" to "sun" to the day's open ranges, each a start and
 * an end as "HH:MM"; a day left out is closed.
 */
export type Weekly = Partial<Record<Weekday, [string, string][]>>;

export type Weekday = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";

export interface Holiday {
  date: string;
  name?: string;
}

export interface Schedule {
  id: string;
  name: string;
  timezone: string;
  is_default: boolean;
  weekly: Weekly;
  holidays: Holiday[];
  status: { open: boolean; next_open: string | null; next_close: string | null };
}

export function schedulesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "schedules"],
    queryFn: () => request<Schedule[]>("GET", "/schedules", undefined, { workspace }),
  });
}

export function updateSchedule(
  workspace: string,
  id: string,
  changes: { name?: string; timezone?: string; weekly?: Weekly; holidays?: Holiday[] },
): Promise<Schedule> {
  return request<Schedule>("PATCH", `/schedules/${encodeURIComponent(id)}`, changes, { workspace });
}
