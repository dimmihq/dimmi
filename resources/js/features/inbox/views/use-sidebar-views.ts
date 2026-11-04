/*
 * The inbox sections of the sidebar that come from the workspace itself:
 * its saved views, its teams, its teammates and, once there are two or
 * more, its inboxes. The views' section offers to make a new one.
 * Each row links to the view of that holder and shows how many open
 * conversations wait there. Teammates sit in a folded section of their
 * own, since a large team would push everything else down. The trash
 * shows only to those who may delete conversations.
 */
import { RiFilter3Line, RiInbox2Line, RiTeamLine, RiUserLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import type { DashboardNavItem, DashboardNavSection } from "@/components/application/dashboard/dashboard-sidebar";
import { countsQuery, inboxesQuery } from "@/lib/api/inbox";
import { teamsQuery } from "@/lib/api/teams";
import { viewsQuery } from "@/lib/api/views";
import { callerQuery, membersQuery } from "@/lib/api/workspace";

type Open = Record<string, { open: number }> | undefined;

function row(key: string, label: string, icon: DashboardNavItem["icon"], id: string, counts: Open): DashboardNavItem {
  const open = counts?.[id]?.open;

  return { key, label, icon, href: `/d/inbox/${id}`, badge: open ? open : undefined };
}

/*
 * sidebarSections builds the sections from what the workspace has.
 */
export function sidebarSections(data: {
  teams?: { id: string; name: string }[];
  members?: { id: string; status: string; user: { name: string } }[];
  inboxes?: { id: string; name: string }[];
  views?: { id: string; name: string }[];
  counts?: { teams: Open; members: Open; inboxes: Open; custom?: Open };
  onNewView?: () => void;
}): DashboardNavSection[] {
  const sections: DashboardNavSection[] = [];
  const { teams = [], members = [], inboxes = [], views = [], counts, onNewView } = data;

  sections.push({
    key: "inbox-views",
    label: "Views",
    items: views.map((v) => row(`inbox-${v.id}`, v.name, RiFilter3Line, v.id, counts?.custom)),
    action: onNewView ? { label: "New view", onClick: onNewView } : undefined,
  });

  if (teams.length > 0) {
    sections.push({
      key: "inbox-teams",
      label: "Teams",
      items: teams.map((t) => row(`inbox-${t.id}`, t.name, RiTeamLine, t.id, counts?.teams)),
    });
  }
  const active = members.filter((m) => m.status === "active");
  if (active.length > 0) {
    sections.push({
      key: "inbox-teammates",
      label: "Teammates",
      items: active.map((m) => row(`inbox-${m.id}`, m.user.name, RiUserLine, m.id, counts?.members)),
    });
  }
  if (inboxes.length >= 2) {
    sections.push({
      key: "inbox-inboxes",
      label: "Inboxes",
      items: inboxes.map((i) => row(`inbox-${i.id}`, i.name, RiInbox2Line, i.id, counts?.inboxes)),
    });
  }

  return sections;
}

/*
 * useSidebarViews returns the workspace's inbox sections and whether the
 * person may see the trash.
 */
export function useSidebarViews(workspace: string) {
  const enabled = workspace !== "";
  const { data: teams } = useQuery({ ...teamsQuery(workspace), enabled });
  const { data: members } = useQuery({ ...membersQuery(workspace), enabled });
  const { data: inboxes } = useQuery({ ...inboxesQuery(workspace), enabled });
  const { data: counts } = useQuery({ ...countsQuery(workspace), enabled });
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled });
  const { data: views } = useQuery({ ...viewsQuery(workspace), enabled });
  const navigate = useNavigate();
  const onNewView = () => void navigate({ to: "/d/inbox/$view", params: { view: "all" }, search: { editor: "new" } });

  return {
    sections: sidebarSections({ teams, members, inboxes, views, counts: counts?.views, onNewView }),
    canDelete: caller?.member?.permissions.includes("conversations:delete") ?? false,
  };
}

/*
 * useViewName returns what the sidebar calls a team, teammate or inbox
 * view, for the list's title.
 */
export function useViewName(workspace: string, view: string): string | undefined {
  const enabled = workspace !== "";
  const { data: teams } = useQuery({ ...teamsQuery(workspace), enabled: enabled && view.startsWith("team_") });
  const { data: members } = useQuery({ ...membersQuery(workspace), enabled: enabled && view.startsWith("mem_") });
  const { data: inboxes } = useQuery({ ...inboxesQuery(workspace), enabled: enabled && view.startsWith("inbox_") });
  const { data: views } = useQuery({ ...viewsQuery(workspace), enabled: enabled && view.startsWith("cvw_") });

  return (
    views?.find((v) => v.id === view)?.name ??
    teams?.find((t) => t.id === view)?.name ??
    members?.find((m) => m.id === view)?.user.name ??
    inboxes?.find((i) => i.id === view)?.name
  );
}
