/*
 * Tabs and accordions render their items to HTML first, then hand the
 * pieces to a hydrated BoardUI component: each item is a custom element
 * whose data-* attributes carry its title and whose body is the panel.
 */
const entities: Record<string, string> = { "&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": '"', "&#39;": "'" };
const decode = (value: string) => value.replace(/&(amp|lt|gt|quot|#39);/g, (match) => entities[match]);

// Stylesheets and scripts a panel carries (code blocks bring their own)
// are lifted out, so the page loads them outside the hydrated component.
const asset = /<link rel="stylesheet"[^>]*>|<script type="module"[^>]*><\/script>/g;

export function splitPanels(source: string, tag: string) {
  const assets = [...new Set(source.match(asset) ?? [])].join("");
  const html = source.replace(asset, "");
  // Astro writes an empty attribute bare (data-description), so the value is optional.
  const pattern = new RegExp(`<${tag}((?:\\s+data-[a-z]+(?:="[^"]*")?)*)>([\\s\\S]*?)</${tag}>`, "g");
  const panels = [...html.matchAll(pattern)].map(([, attrs, body]) => {
    const data = Object.fromEntries(
      [...attrs.matchAll(/data-([a-z]+)(?:="([^"]*)")?/g)].map(([, key, value]) => [key, decode(value ?? "")]),
    );
    return {
      label: data.label ?? "",
      description: data.description || undefined,
      icon: data.icon || undefined,
      html: body.trim(),
    };
  });
  return { panels, assets };
}

export type TreeNode = { id: string; label: string; note?: string; highlight: boolean; children: TreeNode[] };

/*
 * Reads a rendered Markdown list into a tree: each item's first word is
 * its name (a trailing / marks a folder), the rest a note, and a bold
 * name is highlighted.
 */
export function parseList(html: string): TreeNode[] {
  const roots: TreeNode[] = [];
  const lists: TreeNode[][] = [];
  const open: TreeNode[] = [];
  let id = 0;
  let bold = false;
  for (const [, close, tag, text] of html.matchAll(/<(\/?)(ul|li|strong|b)\b[^>]*>|([^<]+)|<[^>]*>/g)) {
    const closing = close === "/";
    if (tag === "ul") {
      closing ? lists.pop() : lists.push(open.at(-1)?.children ?? roots);
    } else if (tag === "li") {
      if (closing) {
        open.pop();
      } else {
        const node: TreeNode = { id: `n${id++}`, label: "", highlight: false, children: [] };
        lists.at(-1)?.push(node);
        open.push(node);
      }
    } else if (tag) {
      bold = !closing;
    } else if (text) {
      addText(open.at(-1), decode(text).trim(), bold);
    }
  }
  return roots;
}

function addText(node: TreeNode | undefined, text: string, bold: boolean) {
  if (!(node && text)) {
    return;
  }
  if (node.label) {
    node.note = [node.note, text].filter(Boolean).join(" ");
    return;
  }
  const [name, ...rest] = text.split(/\s+/);
  node.label = name;
  node.highlight = bold;
  node.note = rest.length ? rest.join(" ") : undefined;
}
