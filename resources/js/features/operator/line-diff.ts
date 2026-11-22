/*
 * A line-by-line diff of two texts, for showing what a suggested edit
 * changes in an article: lines both share, lines it removes and lines it
 * adds, in reading order. Long runs of unchanged lines fold to a few on
 * each side of a change, the way a code review shows them.
 */

export type DiffKind = "same" | "add" | "del" | "fold";

export interface DiffLine {
  kind: DiffKind;
  text: string;
}

/*
 * diffLines compares two texts line by line through their longest
 * common run of lines.
 */
export function diffLines(before: string, after: string): DiffLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  const rows = a.length + 1;
  const cols = b.length + 1;
  const lcs = new Uint32Array(rows * cols);
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i * cols + j] =
        a[i] === b[j] ? lcs[(i + 1) * cols + j + 1] + 1 : Math.max(lcs[(i + 1) * cols + j], lcs[i * cols + j + 1]);
    }
  }
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      out.push({ kind: "same", text: a[i] });
      i++;
      j++;
    } else if (lcs[(i + 1) * cols + j] >= lcs[i * cols + j + 1]) {
      out.push({ kind: "del", text: a[i++] });
    } else {
      out.push({ kind: "add", text: b[j++] });
    }
  }
  while (i < a.length) {
    out.push({ kind: "del", text: a[i++] });
  }
  while (j < b.length) {
    out.push({ kind: "add", text: b[j++] });
  }
  return out;
}

/*
 * folded keeps `context` unchanged lines around each change and folds
 * longer runs of unchanged lines into one marker saying how many.
 */
export function folded(lines: DiffLine[], context = 2): DiffLine[] {
  const near = lines.map((l) => l.kind !== "same");
  const keep = lines.map((_, i) => {
    for (let k = Math.max(0, i - context); k <= Math.min(lines.length - 1, i + context); k++) {
      if (near[k]) {
        return true;
      }
    }
    return false;
  });
  const out: DiffLine[] = [];
  let hidden = 0;
  lines.forEach((line, i) => {
    if (keep[i]) {
      if (hidden > 0) {
        out.push({ kind: "fold", text: hidden === 1 ? "1 unchanged line" : `${hidden} unchanged lines` });
        hidden = 0;
      }
      out.push(line);
    } else {
      hidden++;
    }
  });
  if (hidden > 0) {
    out.push({ kind: "fold", text: hidden === 1 ? "1 unchanged line" : `${hidden} unchanged lines` });
  }
  return out;
}

/*
 * changeCount is how many lines a diff adds and removes.
 */
export function changeCount(lines: DiffLine[]): { added: number; removed: number } {
  return {
    added: lines.filter((l) => l.kind === "add").length,
    removed: lines.filter((l) => l.kind === "del").length,
  };
}
