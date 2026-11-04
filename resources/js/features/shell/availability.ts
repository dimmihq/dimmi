/*
 * The person's availability in the account menu: three rows, the one in
 * force marked, and a dot on their avatar. Choosing a row changes it on
 * the server and reads the caller again, so every screen agrees.
 */
import { RiArrowGoBackLine, RiCheckboxBlankCircleLine, RiMoonLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type { MenuGroup, PresenceStatus } from "@/components/application/dashboard/dashboard-team-menu";
import { type AvailabilityState, availabilityOf, setAvailability } from "@/lib/api/availability";
import { callerQuery } from "@/lib/api/workspace";

const rows: { state: AvailabilityState; label: string; icon: MenuGroup["items"][number]["icon"] }[] = [
  { state: "active", label: "Active", icon: RiCheckboxBlankCircleLine },
  { state: "away", label: "Away", icon: RiMoonLine },
  { state: "away_reassign", label: "Away and reassigning", icon: RiArrowGoBackLine },
];

/*
 * useAvailability returns the menu group and the avatar's dot for the
 * workspace on screen, or nothing outside a workspace.
 */
export function useAvailability(workspace: string | undefined): { group?: MenuGroup; status?: PresenceStatus } {
  const queryClient = useQueryClient();
  const caller = useQuery({ ...callerQuery(workspace ?? ""), enabled: Boolean(workspace) });
  const change = useMutation({
    mutationFn: (state: AvailabilityState) => setAvailability(workspace ?? "", state),
    onSettled: () => queryClient.invalidateQueries({ queryKey: callerQuery(workspace ?? "").queryKey }),
  });

  const member = caller.data?.member;
  if (!workspace || !member) {
    return {};
  }

  const current = availabilityOf(member);

  return {
    status: current === "active" ? "online" : "away",
    group: {
      id: "availability",
      label: "Availability",
      items: rows.map((row) => ({
        icon: row.icon,
        label: row.label,
        isSelected: row.state === current,
        onSelect: () => change.mutate(row.state),
      })),
    },
  };
}
