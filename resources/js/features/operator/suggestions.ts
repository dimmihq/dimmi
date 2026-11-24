/*
 * Which suggestions a page shows: the pending changes Operator made with
 * an operation on the page's kind of content, and how to decide one.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";

import {
  type ChangesetDecision,
  decideChangeset,
  type OperatorChangeset,
  type OperatorOperation,
  pendingChangesetsQuery,
} from "@/lib/api/operator";

/*
 * isArticle reports whether an operation drafts or edits an article.
 */
export const isArticle = (op: OperatorOperation) =>
  op.kind === "article.create_draft" || op.kind === "article.edit_draft";

/*
 * isProcedure reports whether an operation changes a procedure.
 */
export const isProcedure = (op: OperatorOperation) =>
  op.kind === "procedure.edit_draft" || op.kind === "procedure.create_test_case";

/*
 * usePendingCount is how many suggestions wait for review.
 */
export function usePendingCount(workspace: string): number {
  const { data } = useQuery({ ...pendingChangesetsQuery(workspace), enabled: workspace !== "" });

  return (data ?? []).filter((c) => !c.preview).length;
}

/*
 * useSuggestions is the pending changes with an operation the test
 * matches, and how to decide one: deciding refreshes the workspace's
 * data, tells onDecided with the decision and returns the changeset.
 */
export function useSuggestions(
  workspace: string,
  match: (op: OperatorOperation) => boolean,
  onDecided?: (changeset: OperatorChangeset, decision: ChangesetDecision) => void,
) {
  const queryClient = useQueryClient();
  const { data } = useQuery({ ...pendingChangesetsQuery(workspace), enabled: workspace !== "" });
  const found = (data ?? []).filter((c) => !c.preview && c.operations.some(match));
  const decide = async (id: string, decision: ChangesetDecision) => {
    const changeset = await decideChangeset(workspace, id, decision);
    await queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
    onDecided?.(changeset, decision);
    return changeset;
  };
  return { suggestions: found, decide };
}
