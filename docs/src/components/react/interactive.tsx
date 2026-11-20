import { RiFileCodeLine, RiFolder3Line } from "@remixicon/react";

import { Accordion, AccordionItem, type AccordionVariant } from "@/components/base/accordion/accordion";
import { Tab, TabList, TabPanel, Tabs } from "@/components/base/tabs/tabs";
import { TreeView, TreeViewItem } from "@/components/base/tree-view/tree-view";
import { cx } from "@/utils/cx";

/*
 * The hydrated content blocks: tabs, accordion and file tree. Kept apart
 * from content.tsx so the browser never loads the whole icon set.
 */

/* Tabs: BoardUI's underline tabs over prerendered panels. */

export type Panel = { label: string; html: string };

export function DocsTabs({ panels, label = "Tabs" }: { panels: Panel[]; label?: string }) {
  return (
    <Tabs defaultSelectedKey="0" className="not-prose flex flex-col gap-4">
      <TabList aria-label={label}>
        {panels.map((panel, index) => (
          <Tab key={panel.label} id={String(index)}>
            {panel.label}
          </Tab>
        ))}
      </TabList>
      {panels.map((panel, index) => (
        <TabPanel
          key={panel.label}
          id={String(index)}
          shouldForceMount
          className="prose max-w-none outline-none data-[inert]:hidden [&[inert]]:hidden"
        >
          {/* biome-ignore lint/security/noDangerouslySetInnerHtml: the panel is this page's own Markdown, rendered at build time */}
          <div dangerouslySetInnerHTML={{ __html: panel.html }} />
        </TabPanel>
      ))}
    </Tabs>
  );
}

/* Accordion: BoardUI's disclosure group over prerendered panels. */

export function DocsAccordion({
  items,
  variant = "separated",
}: {
  items: (Panel & { description?: string })[];
  variant?: AccordionVariant;
}) {
  return (
    <div className="not-prose">
      <Accordion variant={variant} allowsMultipleExpanded>
        {items.map((item) => (
          <AccordionItem key={item.label} title={item.label} description={item.description}>
            {/* biome-ignore lint/security/noDangerouslySetInnerHtml: the item is this page's own Markdown, rendered at build time */}
            <div className="prose max-w-none" dangerouslySetInnerHTML={{ __html: item.html }} />
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  );
}

/* File tree: an indented outline, drawn in BoardUI's tree view. */

export type Node = { id: string; label: string; note?: string; highlight: boolean; children: Node[] };

function folders(nodes: Node[]): string[] {
  return nodes.flatMap((node) => (node.children.length ? [node.id, ...folders(node.children)] : []));
}

function TreeNode({ node }: { node: Node }) {
  return (
    <TreeViewItem
      id={node.id}
      label={node.label}
      icon={
        node.children.length || node.label.endsWith("/")
          ? node.children.length
            ? undefined
            : RiFolder3Line
          : node.label.match(/\.(tsx?|go|php|py|json|ya?ml)$/)
            ? RiFileCodeLine
            : undefined
      }
      labelContent={
        <span
          className={cx(
            "font-mono text-[0.8125rem]",
            node.highlight ? "text-accent-600 dark:text-accent-400" : "text-text-primary",
          )}
        >
          {node.label}
        </span>
      }
      trailingContent={
        node.note ? <span className="truncate text-caption-1-regular text-text-tertiary">{node.note}</span> : undefined
      }
    >
      {node.children.map((child) => (
        <TreeNode key={child.id} node={child} />
      ))}
    </TreeViewItem>
  );
}

export function FileTree({ nodes }: { nodes: Node[] }) {
  return (
    // TreeView draws its own card, so no frame around it.
    <div className="not-prose">
      <TreeView aria-label="Files" size="sm" selectionMode="none" defaultExpandedKeys={folders(nodes)}>
        {nodes.map((node) => (
          <TreeNode key={node.id} node={node} />
        ))}
      </TreeView>
    </div>
  );
}
