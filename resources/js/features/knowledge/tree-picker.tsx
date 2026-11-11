/*
 * Ticking what a source reads from its own structure, drawn like the
 * help center's collections tree: Intercom collections and articles,
 * Zendesk categories, sections and articles, Notion pages, or a
 * repository's folders and Markdown files. Ticking an item takes
 * everything under it, so its children show as ticked and cannot be
 * unticked on their own; nothing ticked reads everything. The top level
 * opens at first. The tree has no frame of its own and scrolls inside
 * the space it is given, filling a dialog when asked to, with the top
 * edge softened once scrolled. While the service is read, a
 * skeleton of rows holds the place.
 */
import { RiFileTextLine } from "@remixicon/react";
import { useState } from "react";
import type { Key, Selection } from "react-aria-components";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { TreeView, TreeViewItem } from "@/components/base/tree-view/tree-view";
import { ScrollFade, useScrollFade } from "@/components/foundations/scroll-fade/scroll-fade";
import type { TreeNode } from "@/lib/api/knowledge-sources";
import { cx } from "@/utils/cx";

import { inheritedOf, nextSelection } from "./tree-selection";

export function TreePicker({
  nodes,
  selected,
  readOnly = false,
  fill = false,
  onChange,
}: {
  nodes: TreeNode[];
  selected: string[];
  readOnly?: boolean;
  fill?: boolean;
  onChange?: (selected: string[]) => void;
}) {
  const fade = useScrollFade();
  const [expanded, setExpanded] = useState<Set<Key>>(() => new Set(nodes.map((n) => n.id)));
  const inherited = inheritedOf(nodes, selected);

  if (nodes.length === 0) {
    return <p className="text-body-2-regular text-text-tertiary">Nothing to choose from yet.</p>;
  }

  return (
    <div className={cx("relative", fill && "flex min-h-0 flex-1 flex-col")}>
      <div className={cx("overflow-y-auto", fill ? "min-h-0 flex-1" : "max-h-96")} onScroll={fade.onScroll}>
        <TreeView
          aria-label="Items to read"
          size="sm"
          className="rounded-none border-0 bg-transparent p-0"
          selectionMode="multiple"
          selectionBehavior="toggle"
          disabledBehavior="selection"
          disabledKeys={readOnly ? [...allIds(nodes)] : [...inherited]}
          selectedKeys={new Set([...selected, ...inherited])}
          onSelectionChange={(keys: Selection) => {
            if (keys !== "all") {
              onChange?.(nextSelection(nodes, selected, new Set([...keys].map(String))));
            }
          }}
          expandedKeys={expanded}
          onExpandedChange={setExpanded}
        >
          {nodes.map((node) => (
            <Branch key={node.id} node={node} />
          ))}
        </TreeView>
      </div>
      <ScrollFade
        progress={fade.progress}
        surfaceClassName={fill ? "from-background-full" : "from-background-primary-default"}
      />
    </div>
  );
}

function Branch({ node }: { node: TreeNode }) {
  const title = node.title || "Untitled";
  const leaf = node.children.length === 0;

  return (
    <TreeViewItem
      id={node.id}
      label={title}
      icon={leaf ? RiFileTextLine : undefined}
      labelContent={
        <span className="flex min-w-0 flex-1 items-center gap-2">
          <Checkbox slot="selection" size="sm" aria-label={title} />
          <span className="min-w-0 flex-1 truncate text-start">
            <bdi>{title}</bdi>
          </span>
        </span>
      }
    >
      {node.children.map((child) => (
        <Branch key={child.id} node={child} />
      ))}
    </TreeViewItem>
  );
}

function allIds(nodes: TreeNode[]): string[] {
  return nodes.flatMap((n) => [n.id, ...allIds(n.children)]);
}

/*
 * TreeSkeleton holds the tree's place while the service is read.
 */
export function TreeSkeleton() {
  const rows = [
    { id: "a", depth: 0, width: "w-40" },
    { id: "b", depth: 1, width: "w-52" },
    { id: "c", depth: 1, width: "w-36" },
    { id: "d", depth: 0, width: "w-44" },
    { id: "e", depth: 1, width: "w-48" },
    { id: "f", depth: 0, width: "w-32" },
  ];

  return (
    <div aria-hidden className="flex flex-col gap-1">
      {rows.map((row) => (
        <div
          key={row.id}
          className="flex min-h-8 items-center gap-2 px-2"
          style={{ paddingInlineStart: `calc(0.5rem + ${row.depth} * 1.25rem)` }}
        >
          <Skeleton className="size-4 shrink-0 rounded" />
          <Skeleton className="size-4 shrink-0 rounded" />
          <Skeleton shape="text" className={`h-3.5 max-w-full ${row.width}`} />
        </div>
      ))}
    </div>
  );
}
