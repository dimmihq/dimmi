/*
 * Saving a changelog's labels: adding, renaming, recoloring to the next
 * color and deleting, each reloading the labels after. The changelog's
 * pane and its settings page both manage labels with these.
 */
import { RiDeleteBinLine, RiEditLine, RiPaletteLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import type { RowAction } from "@/features/shell/row-menu";
import {
  type ChangelogLabel,
  changelogKeys,
  createLabel,
  deleteLabel,
  labelColors,
  updateLabel,
} from "@/lib/api/changelog";

export type LabelAct = "rename" | "color" | "delete";

export const labelActions: RowAction<LabelAct>[] = [
  { kind: "rename", label: "Rename", icon: RiEditLine },
  { kind: "color", label: "Next color", icon: RiPaletteLine },
  { kind: "delete", label: "Delete", icon: RiDeleteBinLine, danger: true },
];

export function nextColor(color: string): string {
  const at = labelColors.indexOf(color);
  return labelColors[(at + 1) % labelColors.length] ?? labelColors[0] ?? "#64748b";
}

/*
 * useLabels saves label changes and reloads the labels after each.
 */
export function useLabels(workspace: string, helpCenter: string) {
  const client = useQueryClient();
  const refresh = () => client.invalidateQueries({ queryKey: changelogKeys.labels(workspace, helpCenter) });
  const change = useMutation({
    mutationFn: (fn: () => Promise<unknown>) => fn(),
    onSuccess: () => refresh(),
    onError: (error) => toast.error(error.message),
  });
  return {
    add: (name: string) =>
      change.mutateAsync(() => createLabel(workspace, helpCenter, { name, color: labelColors[3] })),
    rename: (id: string, name: string) => change.mutateAsync(() => updateLabel(workspace, id, { name })),
    recolor: (label: ChangelogLabel) =>
      change.mutate(() => updateLabel(workspace, label.id, { color: nextColor(label.color) })),
    remove: (id: string) => change.mutateAsync(() => deleteLabel(workspace, id)),
  };
}
