/*
 * Words for tagging customers in bulk.
 */
import type { TaggedList } from "@/lib/api/audiences";

function people(n: number): string {
  return n === 1 ? "1 person" : `${n.toLocaleString()} people`;
}

/*
 * listSummary says what an upload or a freeze did.
 */
export function listSummary(r: TaggedList): string {
  const parts = [`${people(r.matched)} matched`, `${people(r.tagged)} newly tagged ${r.tag_name}`];
  if (r.unmatched > 0) {
    const shown = r.unmatched_sample.slice(0, 3).join(", ");
    parts.push(`${r.unmatched.toLocaleString()} not found${shown ? ` (${shown}${r.unmatched > 3 ? ", …" : ""})` : ""}`);
  }

  return `${parts.join(", ")}.`;
}
