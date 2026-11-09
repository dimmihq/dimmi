/*
 * A callout as the writer draws it: the same aside readers see, titled
 * by its kind, with the kind picked from the title.
 */
import { NodeViewContent, type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { useState } from "react";

import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { cx } from "@/utils/cx";

import { type CalloutKind, calloutKinds } from "./callout-kinds";

const titles: Record<CalloutKind, string> = {
  note: "Note",
  tip: "Tip",
  important: "Important",
  warning: "Warning",
  caution: "Caution",
};

export function CalloutView({ node, updateAttributes, editor }: NodeViewProps) {
  const kind = node.attrs.kind as CalloutKind;
  const [open, setOpen] = useState(false);

  return (
    <NodeViewWrapper as="aside" data-callout="" className={cx("alert callout", `alert-${kind}`)}>
      <div contentEditable={false} className="callout-title">
        {editor.isEditable ? (
          <Dropdown isOpen={open} onOpenChange={setOpen}>
            <DropdownTrigger aria-label={`Callout kind: ${titles[kind]}`} className="callout-kind">
              {titles[kind]}
            </DropdownTrigger>
            <DropdownPopover aria-label="Callout kind" placement="bottom start" className="w-40 p-1">
              {calloutKinds.map((k) => (
                <DropdownItem
                  key={k}
                  selected={k === kind}
                  onSelect={() => {
                    updateAttributes({ kind: k });
                    setOpen(false);
                  }}
                >
                  <span className={cx("size-2 rounded-full", `callout-dot alert-${k}`)} aria-hidden />
                  <span className="text-body-medium text-text-primary">{titles[k]}</span>
                </DropdownItem>
              ))}
            </DropdownPopover>
          </Dropdown>
        ) : (
          titles[kind]
        )}
      </div>
      <NodeViewContent />
    </NodeViewWrapper>
  );
}
