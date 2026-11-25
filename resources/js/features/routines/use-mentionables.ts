/*
 * Everything a routine's instructions can mention, read once for the
 * editor. A teammate who may not read one kind simply gets no
 * suggestions of it.
 */
import { useQuery } from "@tanstack/react-query";

import { useChipSources } from "@/features/automation/procedure-writer/use-chip-sources";
import { agentsQuery } from "@/lib/api/agents";
import { inboxesQuery } from "@/lib/api/inboxes";
import { tagsQuery } from "@/lib/api/tags";
import { membersQuery } from "@/lib/api/workspace";

import type { Mentionables } from "./mentions";

export function useMentionables(workspace: string): Mentionables {
  const quiet = { retry: false, enabled: workspace !== "" } as const;
  const shared = useChipSources(workspace, "");
  const members = useQuery({ ...membersQuery(workspace), ...quiet }).data ?? [];
  const inboxes = useQuery({ ...inboxesQuery(workspace), ...quiet }).data ?? [];
  const agents = useQuery({ ...agentsQuery(workspace), ...quiet }).data ?? [];
  const tags = useQuery({ ...tagsQuery(workspace), ...quiet }).data ?? [];

  return {
    handoff: shared.teams,
    teammate: members
      .filter((m) => m.status === "active")
      .map((m) => ({ id: m.id, name: m.user.name || m.user.email, avatar: m.user.avatar_url })),
    inbox: inboxes.map((i) => ({ id: i.id, name: i.name })),
    tag: tags.map((t) => ({ id: t.id, name: t.name, color: t.color, icon: t.icon })),
    agent: agents.map((a) => ({ id: a.id, name: a.name, isDefault: a.is_default })),
    article: shared.articles,
    sub_procedure: shared.procedures,
    ticket_type: shared.ticketTypes,
    action: shared.actions.filter((a) => a.status === "published").map((a) => ({ id: a.id, name: a.title || a.name })),
  };
}
