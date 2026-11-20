/*
 * One of the customer's own attributes, its raw key as the label. A
 * star, shown when the pointer is on the row, pins the key for this
 * teammate across every customer, and pinned keys show first, so the
 * plan or account size someone checks on every conversation is the
 * first thing they read.
 */
import { RiStarFill, RiStarLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { show } from "./format";
import { onRowHover } from "./quiet";
import { Copyable, Row } from "./rows";

export function AttributeRow({
  name,
  value,
  pinned,
  onToggle,
}: {
  name: string;
  value: unknown;
  pinned: boolean;
  onToggle: () => void;
}) {
  return (
    <Row
      label={
        <>
          <span className="truncate">{name}</span>
          <Button
            variant="secondary"
            size="xs"
            iconOnly
            leadingIcon={pinned ? RiStarFill : RiStarLine}
            aria-label={pinned ? `Unpin ${name}` : `Pin ${name}`}
            className={cx("shrink-0", onRowHover)}
            onClick={onToggle}
          />
        </>
      }
    >
      <Copyable value={show(value)} label={name} />
    </Row>
  );
}
