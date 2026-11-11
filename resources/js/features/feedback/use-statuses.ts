/*
 * The statuses in the order the workspace chose, for every list and
 * picker of them; the built-in order stands in while it loads.
 */
import { useQuery } from "@tanstack/react-query";

import { type FeedbackStatus, statuses, statusesQuery } from "@/lib/api/feedback";

export function useStatuses(workspace: string): FeedbackStatus[] {
  const { data } = useQuery(statusesQuery(workspace));

  return data ?? statuses;
}
