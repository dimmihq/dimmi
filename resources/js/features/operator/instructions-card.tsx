/*
 * The instructions a routine's run followed, at the top of its thread:
 * a quiet card that shows the first few lines under a fade, and the rest
 * when asked, so the run's report is what the eye lands on.
 */
import { RiArrowDownSLine, RiCalendarScheduleLine } from "@remixicon/react";
import { useLayoutEffect, useRef, useState } from "react";

import { MessageBody } from "@/features/inbox/message-body";
import { cx } from "@/utils/cx";

/* folded is how tall the card stays until opened, about four lines. */
const folded = 112;

export function InstructionsCard({ label, text }: { label: string; text: string }) {
  const [open, setOpen] = useState(false);
  const [long, setLong] = useState(false);
  const body = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    setLong((body.current?.scrollHeight ?? 0) > folded + 8);
  }, []);

  return (
    <section
      aria-label="Instructions"
      className="flex flex-col gap-2 rounded-2xl bg-background-secondary-default px-3.5 pt-3 pb-2.5"
    >
      <header className="flex items-center gap-1.5">
        <RiCalendarScheduleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="text-body-2-medium text-text-secondary">Instructions</span>
        <span className="min-w-0 truncate text-body-2-regular text-text-tertiary">· {label}</span>
      </header>
      <div
        ref={body}
        style={open || !long ? undefined : { maxHeight: folded }}
        className={cx(
          "relative overflow-hidden",
          long &&
            !open &&
            "[mask-image:linear-gradient(to_bottom,black_55%,transparent)] [-webkit-mask-image:linear-gradient(to_bottom,black_55%,transparent)]",
        )}
      >
        <MessageBody text={text} className="text-body-2-regular text-text-secondary" />
      </div>
      {long ? (
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          className="flex w-fit cursor-pointer items-center gap-0.5 rounded-md text-caption-1-medium text-text-tertiary outline-none transition-colors duration-150 hover:text-text-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          {open ? "Show less" : "Show all"}
          <RiArrowDownSLine
            aria-hidden
            className={cx("size-3.5 transition-transform duration-300 ease", open && "rotate-180")}
          />
        </button>
      ) : null}
    </section>
  );
}
