/*
 * A collapsible section as the writer draws it: the chevron folds it the
 * way a reader would, the title is typed in place, and what it holds is
 * written below like any other part of the article. It starts open so
 * nothing in it is hidden from the writer.
 */
import { RiArrowRightSLine } from "@remixicon/react";
import { NodeViewContent, type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { useEffect, useRef, useState } from "react";

import { cx } from "@/utils/cx";

export function DetailsView({ node, updateAttributes, editor, getPos }: NodeViewProps) {
  const [open, setOpen] = useState(true);
  const summary = String(node.attrs.summary ?? "");
  const title = useRef<HTMLInputElement | null>(null);
  const fresh = useRef(summary === "" && editor.isEditable);

  // A section just added starts with its title to type.
  useEffect(() => {
    if (fresh.current) {
      title.current?.focus();
    }
  }, []);

  return (
    <NodeViewWrapper className="details details-writer" data-open={open ? "" : undefined}>
      <div contentEditable={false} className="details-summary">
        <button
          type="button"
          aria-label={open ? "Fold section" : "Unfold section"}
          aria-expanded={open}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => setOpen(!open)}
          className="details-chevron"
        >
          <RiArrowRightSLine aria-hidden className={cx("size-5 transition-transform", open && "rotate-90")} />
        </button>
        <input
          ref={title}
          aria-label="Section title"
          placeholder="Question or title"
          value={summary}
          readOnly={!editor.isEditable}
          onChange={(event) => updateAttributes({ summary: event.target.value })}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === "ArrowDown") {
              event.preventDefault();
              setOpen(true);
              const at = getPos();
              editor.commands.focus(typeof at === "number" ? at + 2 : undefined);
            }
          }}
          className="details-title"
        />
      </div>
      <div className="details-fold" data-closed={open ? undefined : ""}>
        <NodeViewContent className="details-body" />
      </div>
    </NodeViewWrapper>
  );
}
