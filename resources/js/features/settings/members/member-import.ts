/*
 * The words of a member import report: one line for the whole file, one
 * per row that needs the admin's eye. A dry run speaks of what would
 * happen, a real run of what did.
 */
import type { MemberImport } from "@/lib/api/workspace";

/*
 * summary says what the file did, or would do, in one sentence.
 */
export function summary(report: MemberImport): string {
  const parts = [
    `${report.created} to invite`,
    `${report.updated} to change`,
    `${report.unchanged} unchanged`,
    `${report.failed} with problems`,
  ];
  const line = parts.join(", ");

  if (report.dry_run) {
    return `This file has ${line}.`;
  }

  return `Done: ${report.created} invited, ${report.updated} changed, ${report.unchanged} unchanged, ${report.failed} with problems.`;
}

/*
 * problems lists each failed row as "Row 3, ada@acme.com: reason".
 */
export function problems(report: MemberImport): string[] {
  return report.rows
    .filter((row) => row.result === "failed")
    .map((row) => `Row ${row.row}, ${row.email || "no address"}: ${row.error?.message ?? "could not be applied."}`);
}

/*
 * hasWork says whether applying the file would change anything.
 */
export function hasWork(report: MemberImport): boolean {
  return report.created + report.updated > 0;
}
