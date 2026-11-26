/*
 * The directive syntax articles keep their blocks in, read and written
 * the way the help center does (framework/markdown/directives.go): a
 * leaf is one line, ::name[label]{key="value"}, and a container wraps
 * blocks between :::name[label] and a closing fence of at least as many
 * colons, so an outer one holding another has more.
 */

export interface Directive {
  fence: number;
  name: string;
  label: string;
  attrs: Record<string, string>;
}

/*
 * closing returns where the bracket opened at start closes.
 */
function closing(line: string, start: number, open: string, end: string): number {
  let depth = 0;
  for (let i = start; i < line.length; i++) {
    const c = line[i];
    if (c === "\\") {
      i++;
    } else if (c === open) {
      depth++;
    } else if (c === end && --depth === 0) {
      return i;
    }
  }
  return -1;
}

function parseAttrs(text: string): Record<string, string> | null {
  const attrs: Record<string, string> = {};
  const pattern = /\s*([a-z-]+)(?:=(?:"([^"]*)"|'([^']*)'|([^\s"']+)))?/y;
  let at = 0;
  while (at < text.length) {
    if (/^\s*$/.test(text.slice(at))) {
      break;
    }
    pattern.lastIndex = at;
    const match = pattern.exec(text);
    if (!match?.[1]) {
      return null;
    }
    attrs[match[1]] = match[2] ?? match[3] ?? match[4] ?? "";
    at = pattern.lastIndex;
  }
  return attrs;
}

/*
 * parseDirective reads a directive line, or returns null.
 */
export function parseDirective(line: string): Directive | null {
  const text = line.replace(/^ {0,3}/, "").replace(/[ \t\r\n]+$/, "");
  const head = /^(:{2,})([a-z][a-z0-9-]*)/.exec(text);
  if (!head?.[1] || !head[2]) {
    return null;
  }
  let at = head[0].length;
  let label = "";
  if (text[at] === "[") {
    const end = closing(text, at, "[", "]");
    if (end < 0) {
      return null;
    }
    label = text.slice(at + 1, end).replace(/\\([!-/:-@[-`{-~])/g, "$1");
    at = end + 1;
  }
  let attrs: Record<string, string> = {};
  if (text[at] === "{") {
    const end = closing(text, at, "{", "}");
    const parsed = end < 0 ? null : parseAttrs(text.slice(at + 1, end));
    if (!parsed) {
      return null;
    }
    attrs = parsed;
    at = end + 1;
  }
  if (at !== text.length) {
    return null;
  }

  return { fence: head[1].length, name: head[2], label, attrs };
}

/*
 * escapeLabel escapes what would end a label early.
 */
function escapeLabel(label: string): string {
  return label.replace(/[\\[\]]/g, (c) => `\\${c}`);
}

/*
 * formatDirective writes a directive line. Attributes with no value are
 * left out; a double quote in a value is percent-encoded, which no
 * address needs as it is.
 */
export function formatDirective(fence: number, name: string, label: string, attrs: Record<string, string> = {}) {
  const pairs = Object.entries(attrs)
    .filter(([, value]) => value !== "")
    .map(([key, value]) => `${key}="${value.replace(/"/g, "%22")}"`);
  const braces = pairs.length > 0 ? `{${pairs.join(" ")}}` : "";

  return `${":".repeat(fence)}${name}${label ? `[${escapeLabel(label)}]` : ""}${braces}`;
}

/*
 * leafAt reads a leaf directive of the name on the first line of src.
 */
export function leafAt(src: string, name: string): { raw: string; directive: Directive } | null {
  const line = /^[^\n]*(?:\n|$)/.exec(src)?.[0] ?? "";
  const directive = parseDirective(line);
  if (directive?.fence !== 2 || directive.name !== name) {
    return null;
  }

  return { raw: line, directive };
}

/*
 * containerAt reads a container directive of the name at the start of
 * src: its opening line, the source it holds and everything it spans.
 * Fenced code inside never closes it. A container never closed runs to
 * the end, as the help center reads it.
 */
export function containerAt(src: string, name: string): { raw: string; directive: Directive; inner: string } | null {
  const lines = src.split(/(?<=\n)/);
  const directive = parseDirective(lines[0] ?? "");
  if (!directive || directive.fence < 3 || directive.name !== name) {
    return null;
  }
  const end = closingLine(lines, directive.fence);
  const inner = lines.slice(1, end).join("");
  const raw = lines.slice(0, Math.min(end + 1, lines.length)).join("");

  return { raw, directive, inner };
}

/*
 * closingLine finds the line that closes a container opened with the
 * fence, skipping fenced code, or the end when none does.
 */
function closingLine(lines: string[], fence: number): number {
  let code: string | null = null;
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i] ?? "";
    const codeFence = /^ {0,3}(`{3,}|~{3,})/.exec(line)?.[1];
    if (code !== null) {
      if (codeFence?.[0] === code[0] && codeFence.length >= code.length) {
        code = null;
      }
      continue;
    }
    if (codeFence) {
      code = codeFence;
      continue;
    }
    const colons = /^ {0,3}(:+)[ \t]*\r?\n?$/.exec(line)?.[1];
    if (colons && colons.length >= fence) {
      return i;
    }
  }
  return lines.length;
}

/*
 * fenceFor is how many colons a container needs to hold Markdown that
 * may already have containers of its own.
 */
export function fenceFor(inner: string): number {
  let deepest = 2;
  for (const match of inner.matchAll(/^ {0,3}(:{3,})[a-z]/gm)) {
    deepest = Math.max(deepest, match[1]?.length ?? 0);
  }
  return deepest + 1;
}
