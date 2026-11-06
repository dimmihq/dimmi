/*
 * Every view of the inbox, grouped as the view picker shows them: the
 * fixed views first, then the workspace's saved views, its teams, its
 * teammates and, once there are two or more, its inboxes, with spam and
 * the trash last. Each view shows how many conversations wait in it.
 * Unassigned shows only to those who can see conversations nobody holds,
 * and the trash only to those who may delete conversations.
 */
import { RiFilter3Line, RiInbox2Line, RiTeamLine, RiUserLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import type { DashboardNavItem } from "@/components/application/dashboard/dashboard-sidebar";
import { inboxViews, type Page } from "@/features/shell/navigation";
import { type Counts, countsQuery, inboxesQuery } from "@/lib/api/inbox";
import { teamsQuery } from "@/lib/api/teams";
import { viewsQuery } from "@/lib/api/views";
import { callerQuery, membersQuery } from "@/lib/api/workspace";

type Open = Record<string, { open: number }> | undefined;

/*
 * ViewRow is one view in the picker: where it goes and how many wait.
 */
export interface ViewRow {
  id: string;
  label: string;
  icon: DashboardNavItem["icon"];
  count?: number;
}

/*
 * ViewGroup is a labelled run of views.
 */
export interface ViewGroup {
  key: string;
  label: string;
  rows: ViewRow[];
}

function row(id: string, label: string, icon: ViewRow["icon"], counts: Open): ViewRow {
  return { id, label, icon, count: counts?.[id]?.open || undefined };
}

function fixed(page: Page, count: number | undefined): ViewRow {
  return { id: page.href.split("/").pop() ?? "", label: page.label, icon: page.icon, count: count || undefined };
}

/*
 * openCount is how many conversations wait in a view, as the counts
 * report them: open ones, unread mentions, or the person's snoozed.
 */
export function openCount(counts: Counts["views"] | undefined, view: string): number | undefined {
  if (!counts) {
    return undefined;
  }
  switch (view) {
    case "mentions":
      return counts.mentions.unread;
    case "mine":
    case "unassigned":
    case "all":
      return counts[view].open;
    case "snoozed":
      return counts.mine.snoozed;
  }

  return (counts.teams[view] ?? counts.members[view] ?? counts.inboxes[view] ?? counts.custom[view])?.open;
}

function fixedGroups(counts: Counts["views"] | undefined, unassigned: boolean, canDelete: boolean) {
  const inbox = [inboxViews.mine, inboxViews.unassigned, inboxViews.all, inboxViews.mentions, inboxViews.snoozed]
    .filter((page) => unassigned || page !== inboxViews.unassigned)
    .map((page) => fixed(page, openCount(counts, page.href.split("/").pop() ?? "")));
  const more = (canDelete ? [inboxViews.spam, inboxViews.deleted] : [inboxViews.spam]).map((page) =>
    fixed(page, undefined),
  );

  return { inbox: { key: "inbox", label: "Inbox", rows: inbox }, more: { key: "more", label: "More", rows: more } };
}

/*
 * viewGroups builds the picker's groups from what the workspace has,
 * leaving out groups with nothing in them and a single inbox.
 */
export function viewGroups(data: {
  teams?: { id: string; name: string }[];
  members?: { id: string; status: string; user: { name: string } }[];
  inboxes?: { id: string; name: string }[];
  views?: { id: string; name: string }[];
  counts?: Counts["views"];
  unassigned?: boolean;
  canDelete?: boolean;
}): ViewGroup[] {
  const { teams = [], members = [], inboxes = [], views = [], counts } = data;
  const { inbox, more } = fixedGroups(counts, data.unassigned ?? true, data.canDelete ?? false);
  const active = members.filter((m) => m.status === "active");
  const groups: ViewGroup[] = [
    inbox,
    { key: "views", label: "Views", rows: views.map((v) => row(v.id, v.name, RiFilter3Line, counts?.custom)) },
    { key: "teams", label: "Teams", rows: teams.map((t) => row(t.id, t.name, RiTeamLine, counts?.teams)) },
    {
      key: "teammates",
      label: "Teammates",
      rows: active.map((m) => row(m.id, m.user.name, RiUserLine, counts?.members)),
    },
    {
      key: "inboxes",
      label: "Inboxes",
      rows: inboxes.length >= 2 ? inboxes.map((i) => row(i.id, i.name, RiInbox2Line, counts?.inboxes)) : [],
    },
    more,
  ];

  return groups.filter((group) => group.rows.length > 0);
}

/*
 * useViewGroups returns every view the person can open, grouped.
 */
export function useViewGroups(workspace: string, enabled = true): ViewGroup[] {
  const on = enabled && workspace !== "";
  const { data: teams } = useQuery({ ...teamsQuery(workspace), enabled: on });
  const { data: members } = useQuery({ ...membersQuery(workspace), enabled: on });
  const { data: inboxes } = useQuery({ ...inboxesQuery(workspace), enabled: on });
  const { data: counts } = useQuery({ ...countsQuery(workspace), enabled: on });
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: on });
  const { data: views } = useQuery({ ...viewsQuery(workspace), enabled: on });
  const access = caller?.member;
  const unassigned =
    !access || !["assigned", "teams"].includes(access.conversation_access) || access.conversation_access_unassigned;

  return viewGroups({
    teams,
    members,
    inboxes,
    views,
    counts: counts?.views,
    unassigned,
    canDelete: access?.permissions.includes("conversations:delete") ?? false,
  });
}

/*
 * useViewName returns what the picker calls a team, teammate, inbox or
 * saved view, for the list's title.
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

/*
 * useViewTitle returns what a view is called: a fixed view's label, or
 * the name of the team, teammate, inbox or saved view it shows.
 */
export function useViewTitle(workspace: string, view: string): string {
  const fixed = inboxViews[view as keyof typeof inboxViews] as Page | undefined;
  const name = useViewName(workspace, view);

  return fixed?.label ?? name ?? "Conversations";
}
