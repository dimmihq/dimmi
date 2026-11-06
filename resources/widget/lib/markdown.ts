/*
 * The little Markdown the agent and the team write, turned into safe
 * markup for a bubble: paragraphs and line breaks, bold, italic, inline
 * code, code blocks, bulleted and numbered lists, quotes and links. Every
 * character the writer typed is escaped before any tag is added, so raw
 * HTML shows as the text it is. Links open in a new tab, only to http and
 * https addresses; an image shows as its description, never as a
 * picture. Headings read as bold lines, which suits a chat bubble.
 */

/*
 * escapeHtml makes text safe to place in markup and in an attribute.
 */
export function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}

function safeUrl(url: string): boolean {
  return /^https?:\/\/[^\s<>"'`]+$/i.test(url);
}

function anchor(url: string, label: string): string {
  return `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer">${label}</a>`;
}

/*
 * spans matches, in order of precedence: inline code, an image, a link,
 * a bare address, bold, italic with stars, italic with underscores
 * (only at word edges, so snake_case stays as written).
 */
const spans =
  /`([^`\n]+)`|!\[([^\]\n]*)\]\(([^)\s]*)\)|\[([^\]\n]+)\]\(([^)\s]+)\)|(https?:\/\/[^\s<]*[^\s<.,:;"')\]!?])|\*\*(\S(?:[^\n]*?\S)?)\*\*|__(\S(?:[^\n]*?\S)?)__|\*(\S(?:[^*\n]*?\S)?)\*|(^|[^A-Za-z0-9_])_(\S(?:[^_\n]*?\S)?)_(?![A-Za-z0-9_])/g;

/*
 * inline renders one line's spans.
 */
export function inline(source: string): string {
  let out = "";
  let at = 0;
  for (const m of source.matchAll(spans)) {
    out += escapeHtml(source.slice(at, m.index));
    at = (m.index ?? 0) + m[0].length;
    out += span(m);
  }

  return out + escapeHtml(source.slice(at));
}

function span(m: RegExpMatchArray): string {
  const [, code, alt, , text, url, bare, strong, strong2, em, prefix, em2] = m;
  if (code !== undefined) {
    return `<code>${escapeHtml(code)}</code>`;
  }
  if (alt !== undefined) {
    return escapeHtml(alt);
  }
  if (text !== undefined && url !== undefined) {
    return safeUrl(url) ? anchor(url, inline(text)) : inline(text);
  }
  if (bare !== undefined) {
    return anchor(bare, escapeHtml(bare));
  }
  if (strong !== undefined || strong2 !== undefined) {
    return `<strong>${inline(strong ?? strong2 ?? "")}</strong>`;
  }
  if (em !== undefined) {
    return `<em>${inline(em)}</em>`;
  }

  return `${escapeHtml(prefix ?? "")}<em>${inline(em2 ?? "")}</em>`;
}

const bullet = /^\s*[-*+]\s+(.*)$/;
const numbered = /^\s*\d{1,9}[.)]\s+(.*)$/;
const quote = /^\s*>\s?(.*)$/;
const heading = /^\s*#{1,6}\s+(.*?)\s*#*\s*$/;
const fence = /^\s*(```|~~~)/;

/*
 * Block is one run of lines that renders as a unit.
 */
interface Block {
  kind: "p" | "ul" | "ol" | "quote" | "code";
  lines: string[];
}

function kindOf(line: string): Block["kind"] {
  if (bullet.test(line)) {
    return "ul";
  }
  if (numbered.test(line)) {
    return "ol";
  }

  return quote.test(line) ? "quote" : "p";
}

/*
 * blocks splits the text into paragraphs, lists, quotes and code.
 */
function blocks(text: string): Block[] {
  const out: Block[] = [];
  let open: Block | undefined;
  let code: Block | undefined;
  for (const line of text.replace(/\r\n?/g, "\n").split("\n")) {
    if (code) {
      if (fence.test(line)) {
        code = undefined;
      } else {
        code.lines.push(line);
      }
      continue;
    }
    if (fence.test(line)) {
      code = { kind: "code", lines: [] };
      out.push(code);
      open = undefined;
      continue;
    }
    if (line.trim() === "") {
      open = undefined;
      continue;
    }
    const kind = kindOf(line);
    if (!open || open.kind !== kind) {
      open = { kind, lines: [] };
      out.push(open);
    }
    open.lines.push(line);
  }

  return out;
}

function paragraph(lines: string[]): string {
  return lines
    .map((line) => {
      const title = heading.exec(line);
      return title ? `<strong>${inline(title[1] ?? "")}</strong>` : inline(line.trim());
    })
    .join("<br>");
}

function render(block: Block): string {
  switch (block.kind) {
    case "code":
      return `<pre><code>${escapeHtml(block.lines.join("\n"))}</code></pre>`;
    case "ul":
      return `<ul>${block.lines.map((l) => `<li>${inline(bullet.exec(l)?.[1] ?? l)}</li>`).join("")}</ul>`;
    case "ol":
      return `<ol>${block.lines.map((l) => `<li>${inline(numbered.exec(l)?.[1] ?? l)}</li>`).join("")}</ol>`;
    case "quote":
      return `<blockquote>${paragraph(block.lines.map((l) => quote.exec(l)?.[1] ?? l))}</blockquote>`;
    default:
      return `<p>${paragraph(block.lines)}</p>`;
  }
}

/*
 * markdown renders a message's text as safe markup.
 */
export function markdown(text: string): string {
  return blocks(text).map(render).join("");
}
