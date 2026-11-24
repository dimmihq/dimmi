/*
 * How a requested change reads against the article: the article's blocks
 * and the requested blocks compared in order, so each block is kept,
 * removed, or added after the block before it. Blocks compare by what
 * they hold, so a reworded paragraph reads as the old one removed and
 * the new one added beside it.
 */

export type BlockChange =
  | { kind: "same"; from: number; to: number }
  | { kind: "del"; from: number }
  | {
      kind: "add";
      to: number;
    };

/*
 * commonRuns is, for each pair of places, the longest run the two lists
 * still share from there.
 */
function commonRuns(before: string[], after: string[]): number[][] {
  const common: number[][] = Array.from({ length: before.length + 1 }, () =>
    new Array<number>(after.length + 1).fill(0),
  );
  for (let i = before.length - 1; i >= 0; i--) {
    for (let j = after.length - 1; j >= 0; j--) {
      common[i][j] = before[i] === after[j] ? common[i + 1][j + 1] + 1 : Math.max(common[i + 1][j], common[i][j + 1]);
    }
  }
  return common;
}

/*
 * diffBlocks compares two lists of block keys with a longest common
 * run, removals before additions where a block was replaced.
 */
export function diffBlocks(before: string[], after: string[]): BlockChange[] {
  const n = before.length;
  const m = after.length;
  const common = commonRuns(before, after);
  const out: BlockChange[] = [];
  let i = 0;
  let j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && before[i] === after[j]) {
      out.push({ kind: "same", from: i++, to: j++ });
    } else if (i < n && (j >= m || common[i + 1][j] >= common[i][j + 1])) {
      out.push({ kind: "del", from: i++ });
    } else {
      out.push({ kind: "add", to: j++ });
    }
  }
  return out;
}

/*
 * changedCount is how many blocks a request adds and removes.
 */
export function changedCount(changes: BlockChange[]): { added: number; removed: number } {
  return {
    added: changes.filter((c) => c.kind === "add").length,
    removed: changes.filter((c) => c.kind === "del").length,
  };
}
