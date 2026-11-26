/*
 * What a source with structure reads, changed after it was added: the
 * same tree as when it was added, with the items ticked now. Saving the
 * new ticks starts a sync, which pulls in what was added and removes
 * what was left out.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { Section } from "@/features/settings/layout";
import { ApiError } from "@/lib/api/client";
import {
  type KnowledgeSource,
  type SourceType,
  sourceTreeQuery,
  type TreeNode,
  updateSource,
} from "@/lib/api/knowledge-sources";

import { chosen, countChosen, pagesLine, patternTree } from "./model";
import { TreePicker, TreeSkeleton } from "./tree-picker";

export function ContentSection({
  workspace,
  source,
  kind,
  canManage,
}: {
  workspace: string;
  source: KnowledgeSource;
  kind: SourceType;
  canManage: boolean;
}) {
  const client = useQueryClient();
  const tree = useQuery(sourceTreeQuery(workspace, source.id));
  const patterns = patternTree(kind);
  const saved = patterns ? source.include : chosen(source.settings);
  const [selected, setSelected] = useState(saved);
  const changed = [...selected].sort().join() !== [...saved].sort().join();

  const save = useMutation({
    mutationFn: () =>
      updateSource(
        workspace,
        source.id,
        patterns ? { include: selected } : { settings: { ...source.settings, selected } },
      ),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
      toast.success("Saved. A sync is bringing in your changes.");
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "The selection could not be saved."),
  });

  return (
    <Section
      title="Content"
      description={readsLine(tree.data, selected, patterns)}
      action={
        canManage && changed ? (
          <Button size="small" loading={save.isPending} onClick={() => save.mutate()}>
            Save and sync
          </Button>
        ) : null
      }
    >
      <div className="px-1 py-1">
        {tree.isPending ? (
          <TreeSkeleton />
        ) : (
          <TreePicker nodes={tree.data ?? []} selected={selected} readOnly={!canManage} onChange={setSelected} />
        )}
      </div>
    </Section>
  );
}

/*
 * readsLine says how much the ticks read: for a website, within its
 * page limit; for anything else, the documents chosen.
 */
function readsLine(nodes: TreeNode[] | undefined, selected: string[], pages: boolean): string | undefined {
  if (!nodes) {
    return undefined;
  }
  const count = countChosen(nodes, selected);

  return pages ? pagesLine(count, selected.length) : `${count} documents chosen`;
}
