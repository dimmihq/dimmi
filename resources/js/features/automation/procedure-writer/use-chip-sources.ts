/*
 * Everything a procedure's chips can name, read once for the editor:
 * published actions, customer and company attributes, published help
 * articles of every help center, teams, tags, ticket types and the other
 * procedures that can be followed as sub-procedures. A teammate who may
 * not read one of them simply gets no suggestions of that kind.
 */
import { useQueries, useQuery } from "@tanstack/react-query";

import { actionsQuery } from "@/lib/api/ai-actions";
import { attributesQuery } from "@/lib/api/attributes";
import { articlesQuery, helpCentersQuery } from "@/lib/api/help-centers";
import { proceduresQuery } from "@/lib/api/procedures";
import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import { ticketTypesQuery } from "@/lib/api/tickets";

import type { ChipSources } from "./procedure-suggestions";

export function useChipSources(workspace: string, self: string): ChipSources {
  const quiet = { retry: false } as const;
  const actions = useQuery({ ...actionsQuery(workspace), ...quiet }).data ?? [];
  const attributes = useQuery({ ...attributesQuery(workspace), ...quiet }).data ?? [];
  const teams = useQuery({ ...teamsQuery(workspace), ...quiet }).data ?? [];
  const tags = useQuery({ ...tagsQuery(workspace), ...quiet }).data ?? [];
  const ticketTypes = useQuery({ ...ticketTypesQuery(workspace), ...quiet }).data ?? [];
  const procedures = useQuery({ ...proceduresQuery(workspace), ...quiet }).data ?? [];
  const centers = useQuery({ ...helpCentersQuery(workspace), ...quiet }).data ?? [];
  const articles = useQueries({
    queries: centers.slice(0, 5).map((center) => ({ ...articlesQuery(workspace, center.id), ...quiet })),
  });

  return {
    actions,
    attributes,
    teams: teams.map((t) => ({ id: t.id, name: t.name })),
    tags: tags.map((t) => ({ id: t.id, name: t.name })),
    ticketTypes: ticketTypes.map((t) => ({ id: t.id, name: t.name })),
    procedures: procedures.filter((p) => p.id !== self && p.status === "live").map((p) => ({ id: p.id, name: p.name })),
    articles: articles
      .flatMap((q) => q.data ?? [])
      .filter((a) => a.status === "published")
      .map((a) => ({ id: a.id, name: a.title })),
  };
}
