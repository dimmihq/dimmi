/*
 * A change Operator suggests to a series, read against the working copy:
 * which steps it adds, changes and removes, which paths it joins or
 * cuts, and whether it changes who enters, what makes them leave or the
 * goal. The builder draws the suggested graph with those marks, so the
 * teammate reviews it on the canvas and accepts or declines it there.
 * Accepting saves the suggested copy as the working copy.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "@/components/base/toast/toast";
import { stableJSON } from "@/features/automation/studio/workflow-review";
import { useSuggestions } from "@/features/operator/suggestions";
import type { OperatorChangeset, OperatorOperation } from "@/lib/api/operator";
import { type Series, type SeriesEdge, type SeriesSpec, seriesQuery } from "@/lib/api/series";

export interface SeriesRequest {
  changeset: OperatorChangeset;
  edit: OperatorOperation;
}

export interface SeriesReview {
  proposed: SeriesSpec;
  added: Set<string>;
  changed: Set<string>;
  removed: Set<string>;
  // joined and cut are paths, as `from:path`.
  joined: Set<string>;
  cut: Set<string>;
  // head is true when the entry, exit, goal or quiet hours change.
  head: boolean;
}

const pathKey = (e: SeriesEdge) => `${e.from}:${e.path || "next"}`;
const edgeKey = (e: SeriesEdge) => `${pathKey(e)}>${e.to}`;
const headOf = (s: SeriesSpec) =>
  stableJSON({ entry: s.entry, exit: s.exit, goal: s.goal ?? null, quiet: s.quiet_hours ?? false });

/*
 * reviewOf reads a suggested graph against the working copy.
 */
export function reviewOf(current: SeriesSpec, proposed: SeriesSpec): SeriesReview {
  const before = new Map(current.nodes.map((n) => [n.id, stableJSON({ kind: n.kind, settings: n.settings ?? {} })]));
  const after = new Map(proposed.nodes.map((n) => [n.id, stableJSON({ kind: n.kind, settings: n.settings ?? {} })]));
  const was = new Set(current.edges.map(edgeKey));
  const now = new Set(proposed.edges.map(edgeKey));
  return {
    proposed,
    added: new Set([...after.keys()].filter((id) => !before.has(id))),
    removed: new Set([...before.keys()].filter((id) => !after.has(id))),
    changed: new Set([...after.keys()].filter((id) => before.has(id) && before.get(id) !== after.get(id))),
    joined: new Set(proposed.edges.filter((e) => !was.has(edgeKey(e))).map(pathKey)),
    cut: new Set(current.edges.filter((e) => !now.has(edgeKey(e))).map(pathKey)),
    head: headOf(current) !== headOf(proposed),
  };
}

/*
 * requestFor is the oldest pending suggestion on a series.
 */
export function requestFor(changesets: OperatorChangeset[], series: string): SeriesRequest | null {
  const ours = (c: OperatorChangeset) =>
    c.operations.find((o) => o.series_id === series && o.kind === "series.edit_draft");
  const changeset = changesets.filter((c) => ours(c)).sort((a, b) => a.created_at.localeCompare(b.created_at))[0];
  const edit = changeset ? ours(changeset) : undefined;
  return changeset && edit ? { changeset, edit } : null;
}

/*
 * proposedOf is a suggestion's graph, or nothing when it cannot be read.
 */
export function proposedOf(edit: OperatorOperation | undefined): SeriesSpec | null {
  if (!edit?.text) {
    return null;
  }
  try {
    return JSON.parse(edit.text) as SeriesSpec;
  } catch {
    return null;
  }
}

/*
 * useSeriesRequest is the suggestion waiting on a series, read against
 * the working copy the builder shows, and how to accept or decline it.
 * Accepting reads the saved series again and hands it to onAccepted, so
 * the builder can take it as its working copy and lock version.
 */
export function useSeriesRequest(
  workspace: string,
  series: string,
  current: SeriesSpec | undefined,
  onAccepted?: (next: Series) => void,
) {
  const queryClient = useQueryClient();
  const { suggestions, decide } = useSuggestions(workspace, (op) => op.series_id === series);
  const request = requestFor(suggestions, series);
  const proposed = proposedOf(request?.edit);
  const deciding = useMutation({
    mutationFn: async (accepting: boolean) => {
      if (!request) {
        return;
      }
      await decide(request.changeset.id, accepting ? "applications" : "declines");
      if (accepting) {
        onAccepted?.(await queryClient.fetchQuery({ ...seriesQuery(workspace, series), staleTime: 0 }));
      }
    },
    onSuccess: (_, accepting) =>
      toast.success(accepting ? "Changes accepted into the working copy. Publish when ready." : "Suggestion declined."),
    onError: (error) => toast.error(error.message),
  });

  return {
    request,
    review: proposed && current ? reviewOf(current, proposed) : null,
    accept: () => deciding.mutate(true),
    decline: () => deciding.mutate(false),
    deciding: deciding.isPending,
  };
}
