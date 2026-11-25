/*
 * Where a change to content is reviewed: an article's or a procedure's
 * own editor, or the page of a requested new article. Changes to other
 * things have no editor and are decided on their card.
 */
import type { OperatorChangeset, OperatorOperation } from "@/lib/api/operator";

export type ReviewTarget =
  | { to: "/d/help-center/$article"; params: { article: string } }
  | { to: "/d/help-center/requests/$changeset"; params: { changeset: string } }
  | { to: "/d/procedures/$procedure"; params: { procedure: string } };

const contentKinds = new Set([
  "article.create_draft",
  "article.edit_draft",
  "procedure.edit_draft",
  "procedure.create_test_case",
]);

/*
 * isContent reports whether an operation changes an article or a
 * procedure.
 */
export const isContent = (op: OperatorOperation) => contentKinds.has(op.kind);

/*
 * reviewTarget is where a changeset is reviewed, or nothing when it
 * changes more than content or is a dry run preview.
 */
export function reviewTarget(changeset: OperatorChangeset): ReviewTarget | null {
  if (changeset.preview || changeset.operations.length === 0 || !changeset.operations.every(isContent)) {
    return null;
  }
  const op = changeset.operations[0];
  if (op.procedure_id) {
    return { to: "/d/procedures/$procedure", params: { procedure: op.procedure_id } };
  }
  const article = op.created_id ?? op.article_id;
  if (article) {
    return { to: "/d/help-center/$article", params: { article } };
  }
  return { to: "/d/help-center/requests/$changeset", params: { changeset: changeset.id } };
}
