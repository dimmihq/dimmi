/*
 * The tags and saved audiences a rule can pick, with their names, so a
 * rule builder offers them and a summary reads them.
 */
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { audiencesQuery } from "@/lib/api/audiences";
import { tagsQuery } from "@/lib/api/tags";

import type { Lookups } from "./rule-groups";

/*
 * useLookups names the tags and other audiences a rule can pick.
 */
export function useLookups(workspace: string, own: string): { lookups: Lookups; names: Record<string, string> } {
  const tags = useQuery(tagsQuery(workspace)).data ?? [];
  const saved = useQuery(audiencesQuery(workspace)).data ?? [];

  return useMemo(() => {
    const lookups: Lookups = {
      tags: tags.map((t): [string, string] => [t.id, t.name]),
      audiences: saved.filter((a) => a.id !== own).map((a): [string, string] => [a.id, a.name]),
    };

    return { lookups, names: Object.fromEntries([...lookups.tags, ...lookups.audiences]) };
  }, [tags, saved, own]);
}
