/*
 * What every part of the details panel is made of, all of it BoardUI: a
 * section is an item of the panel's plain accordion, with a count when
 * it lists things. A row is a label beside a value that starts at the
 * same edge on every row, so the panel reads as a list and not as a
 * form. A value that can be copied shows its small copy button when the
 * pointer is on its row.
 */
import { RiArrowDownSLine, RiArrowUpSLine, RiCheckLine, RiFileCopyLine } from "@remixicon/react";
import { type ComponentProps, type ReactNode, useEffect, useState } from "react";

import { AccordionItem } from "@/components/base/accordion/accordion";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Textarea } from "@/components/base/textarea/textarea";
import { cx } from "@/utils/cx";

import { onRowHover } from "./quiet";

export function Section({
  title,
  count,
  aside,
  children,
}: {
  title: string;
  /* How many things the section lists, shown beside its title. */
  count?: number;
  /* What sits at the heading's far edge, such as a reference. */
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <AccordionItem
      id={title}
      title={
        <span className="flex items-center gap-1.5">
          {title}
          {count === undefined ? null : <span className="text-text-tertiary tabular-nums">{count}</span>}
          {aside ? <span className="ms-auto text-body-2-regular text-text-tertiary">{aside}</span> : null}
        </span>
      }
    >
      {children}
    </AccordionItem>
  );
}

export function Row({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="group/row flex min-h-8 items-center gap-3">
      <dt className="flex w-32 shrink-0 items-center gap-1 text-body-2-regular text-text-tertiary">{label}</dt>
      <dd className="flex min-w-0 flex-1 items-center gap-1.5 text-body-2-regular text-text-primary">{children}</dd>
    </div>
  );
}

/*
 * Rows lists a section's rows and shows only the first few until the
 * teammate asks for the rest, so a customer with thirty attributes does
 * not push everything else off the panel.
 */
export function Rows({ rows, most = 5 }: { rows: ReactNode[]; most?: number }) {
  const [all, setAll] = useState(false);
  const hidden = rows.length - most;

  return (
    <div className="flex flex-col items-start gap-1">
      <dl className="flex w-full flex-col">{all ? rows : rows.slice(0, most)}</dl>
      {hidden > 0 ? (
        <LinkButton
          variant="secondary"
          size="xs"
          leadingIcon={all ? RiArrowUpSLine : RiArrowDownSLine}
          aria-expanded={all}
          onClick={() => setAll((was) => !was)}
        >
          {all ? "Show less" : `Show ${hidden} more`}
        </LinkButton>
      ) : null}
    </div>
  );
}

/*
 * QuietTextarea is text a teammate writes in place: BoardUI's textarea
 * without its fill until the pointer is on it or it is typed in, growing
 * with what is written.
 */
export function QuietTextarea(props: Omit<ComponentProps<typeof Textarea>, "rows" | "autoResize" | "fieldClassName">) {
  return (
    <div className="-ms-1.5 min-w-0 flex-1">
      <Textarea
        rows={1}
        autoResize
        maxRows={8}
        {...props}
        className={cx("text-body-2-regular", props.className)}
        fieldClassName="bg-transparent px-0.5 py-1.5 ring-1 hover:bg-background-secondary-hover focus-within:bg-background-secondary-hover"
      />
    </div>
  );
}

export function Nothing({ children }: { children: ReactNode }) {
  return <p className="text-body-2-regular text-text-tertiary">{children}</p>;
}

/*
 * Copyable shows a value cut to the row's width, with a small button
 * that puts the whole value on the clipboard and says so for a moment.
 * Values such as addresses and ids keep their own left-to-right order in
 * any layout direction.
 */
export function Copyable({ value, label }: { value: string; label: string }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = setTimeout(() => setCopied(false), 1500);

    return () => clearTimeout(timer);
  }, [copied]);

  return (
    <>
      <span dir="auto" className="min-w-0 truncate" title={value}>
        {value}
      </span>
      <Button
        variant="secondary"
        size="xs"
        iconOnly
        leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
        aria-label={`Copy ${label}`}
        className={cx("shrink-0", copied ? undefined : onRowHover)}
        onClick={() => {
          void navigator.clipboard?.writeText(value);
          setCopied(true);
        }}
      />
    </>
  );
}
