/*
 * Reading saved replies from a spreadsheet saved as CSV: a header row
 * naming title, body and, when wanted, shortcut and shared, then a reply
 * per row. Fields may be quoted, so a body can hold commas, quotes and
 * line breaks. Shared reads yes, true, 1 or shared as shared.
 */
import type { ImportedReply } from "@/lib/api/saved-replies";

/*
 * Reader walks CSV text a character at a time, keeping the row and the
 * field it is in and whether that field is quoted.
 */
interface Reader {
  rows: string[][];
  row: string[];
  field: string;
  quoted: boolean;
}

function endField(r: Reader) {
  r.row.push(r.field);
  r.field = "";
}

function endRow(r: Reader) {
  endField(r);
  r.rows.push(r.row);
  r.row = [];
}

/*
 * readQuoted takes a character inside quotes and returns how many it
 * used: a doubled quote is one quote, a single one ends the quotes.
 */
function readQuoted(r: Reader, char: string, next: string | undefined): number {
  if (char === '"' && next === '"') {
    r.field += '"';
    return 2;
  }
  if (char === '"') {
    r.quoted = false;
  } else {
    r.field += char;
  }

  return 1;
}

/*
 * readPlain takes a character outside quotes and returns how many it
 * used, a CRLF counting as one line break.
 */
function readPlain(r: Reader, char: string, next: string | undefined): number {
  if (char === '"') {
    r.quoted = true;
  } else if (char === ",") {
    endField(r);
  } else if (char === "\n" || char === "\r") {
    endRow(r);
    return char === "\r" && next === "\n" ? 2 : 1;
  } else {
    r.field += char;
  }

  return 1;
}

/*
 * parseCsv splits CSV text into rows of fields, leaving out blank rows.
 */
export function parseCsv(text: string): string[][] {
  const source = text.replace(/^\uFEFF/, "");
  const r: Reader = { rows: [], row: [], field: "", quoted: false };
  let i = 0;
  while (i < source.length) {
    const char = source[i] ?? "";
    const next = source[i + 1];
    i += r.quoted ? readQuoted(r, char, next) : readPlain(r, char, next);
  }
  if (r.field !== "" || r.row.length > 0) {
    endRow(r);
  }

  return r.rows.filter((row) => row.some((value) => value.trim() !== ""));
}

/*
 * CsvReplies is what a file came to: the replies it holds, or why it
 * cannot be read.
 */
export type CsvReplies = { replies: ImportedReply[]; error: null } | { replies: []; error: string };

const yes = ["yes", "true", "1", "shared", "y"];

/*
 * readReplies reads replies from CSV text whose header names the columns.
 */
export function readReplies(text: string): CsvReplies {
  const [header, ...rows] = parseCsv(text);
  const columns = (header ?? []).map((name) => name.trim().toLowerCase());
  const title = columns.indexOf("title");
  const body = columns.indexOf("body");
  if (title < 0 || body < 0) {
    return { replies: [], error: "The first row must name a title and a body column." };
  }
  const shortcut = columns.indexOf("shortcut");
  const shared = columns.indexOf("shared");
  const replies = rows
    .map((row) => ({
      title: (row[title] ?? "").trim(),
      body: (row[body] ?? "").trim(),
      shortcut: shortcut >= 0 ? (row[shortcut] ?? "").trim() || undefined : undefined,
      shared: shared >= 0 && yes.includes((row[shared] ?? "").trim().toLowerCase()),
    }))
    .filter((reply) => reply.title !== "" && reply.body !== "");
  if (replies.length === 0) {
    return { replies: [], error: "No rows with both a title and a body." };
  }
  if (replies.length > 500) {
    return { replies: [], error: "Import at most 500 replies at a time." };
  }

  return { replies, error: null };
}
