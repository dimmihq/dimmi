/*
 * A picture as the writer draws it, sized and placed with the same
 * classes the help center uses. Pulling the handle on either side
 * resizes it as a share of the column, in steps of five, and lets go
 * only once the pointer is released, so the article changes once per
 * resize. A centered picture grows on both sides, so the handle moves
 * twice as far. The picture itself can be dragged to move it.
 */
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { type PointerEvent as ReactPointerEvent, useRef, useState } from "react";

import { cx } from "@/utils/cx";

import { type ImageAlign, layoutClasses, roundWidth } from "./image-layout";

type Side = "left" | "right";

/*
 * Handle is one of the grips on a picture's sides.
 */
function Handle({ side, onGrab }: { side: Side; onGrab: (side: Side, event: ReactPointerEvent) => void }) {
  return (
    <span
      aria-hidden
      data-image-handle={side}
      onPointerDown={(event) => onGrab(side, event)}
      className={cx(
        "absolute top-1/2 z-10 h-12 w-1.5 -translate-y-1/2 cursor-ew-resize rounded-full",
        "border border-background-primary-default bg-foreground-icon-secondary opacity-0 shadow-xs transition-opacity",
        "group-hover/image:opacity-100 in-[.ProseMirror-selectednode]:opacity-100",
        side === "left" ? "left-2" : "right-2",
      )}
    />
  );
}

export function ImageView({ node, updateAttributes, editor }: NodeViewProps) {
  const frame = useRef<HTMLDivElement | null>(null);
  const [dragging, setDragging] = useState<number | null>(null);
  const align = (node.attrs.align as ImageAlign | null) ?? null;
  const width = dragging ?? (node.attrs.width as number | null);

  const grab = (side: Side, event: ReactPointerEvent) => {
    const box = frame.current;
    const column = box?.closest(".ProseMirror")?.clientWidth ?? 0;
    if (!box || column === 0) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const startX = event.clientX;
    const startWidth = box.getBoundingClientRect().width;
    const growth = align === "center" ? 2 : 1;
    let last = roundWidth((startWidth / column) * 100);
    const move = (e: PointerEvent) => {
      const moved = (side === "right" ? e.clientX - startX : startX - e.clientX) * growth;
      last = roundWidth(((startWidth + moved) / column) * 100);
      setDragging(last);
    };
    const release = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", release);
      setDragging(null);
      updateAttributes({ width: last });
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", release);
  };

  return (
    <NodeViewWrapper
      ref={frame}
      className={cx("article-image group/image", layoutClasses({ width, align }))}
      data-resizing={dragging === null ? undefined : ""}
    >
      <img
        src={node.attrs.src}
        alt={node.attrs.alt ?? ""}
        title={node.attrs.title ?? undefined}
        data-uploading={node.attrs.upload ? "" : undefined}
        data-drag-handle=""
        draggable
      />
      {editor.isEditable ? (
        <>
          <Handle side="left" onGrab={grab} />
          <Handle side="right" onGrab={grab} />
        </>
      ) : null}
      {dragging === null ? null : (
        <span className="pointer-events-none absolute top-2 right-2 rounded-md bg-background-primary-default px-1.5 py-0.5 text-caption-1-semibold text-text-secondary shadow-xs">
          {dragging}%
        </span>
      )}
    </NodeViewWrapper>
  );
}
