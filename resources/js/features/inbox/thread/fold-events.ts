/*
 * Folding a thread's events away. With events hidden, each run of events
 * side by side becomes one line saying how many there were, so the
 * messages around them read as one exchange.
 */
import type { ThreadEntry } from "../conversations";

export type FoldedEntry = ThreadEntry | { kind: "folded"; id: string; count: number };

export function foldEvents(entries: ThreadEntry[], hide: boolean): FoldedEntry[] {
  if (!hide) {
    return entries;
  }
  const out: FoldedEntry[] = [];
  for (const entry of entries) {
    const last = out.at(-1);
    if (entry.kind !== "event") {
      out.push(entry);
    } else if (last?.kind === "folded") {
      out[out.length - 1] = { ...last, count: last.count + 1 };
    } else {
      out.push({ kind: "folded", id: `folded-${entry.id}`, count: 1 });
    }
  }

  return out;
}

export function foldedText(count: number): string {
  return count === 1 ? "1 event" : `${count} events`;
}
