/*
 * Page content links to other pages by their path inside the docs, such
 * as [Quickstart](/quickstart/) or <Card href="/api-reference/">. The docs
 * are served under /docs, so this adds that prefix to every root-relative
 * href, in Markdown links and in the href of MDX components alike.
 */

const prefix = (href, base) =>
  typeof href === "string" &&
  href.startsWith("/") &&
  !href.startsWith("//") &&
  href !== base &&
  !href.startsWith(`${base}/`)
    ? `${base}${href}`
    : href;

function visit(node, base) {
  if (node.type === "element" && node.properties?.href) {
    node.properties.href = prefix(node.properties.href, base);
  }
  if ((node.type === "mdxJsxFlowElement" || node.type === "mdxJsxTextElement") && node.attributes) {
    for (const attribute of node.attributes) {
      if (attribute.type === "mdxJsxAttribute" && attribute.name === "href") {
        attribute.value = prefix(attribute.value, base);
      }
    }
  }
  for (const child of node.children ?? []) {
    visit(child, base);
  }
}

export function rehypeBase({ base }) {
  return (tree) => visit(tree, base);
}
