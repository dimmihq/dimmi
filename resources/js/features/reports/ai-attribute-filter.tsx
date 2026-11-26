/*
 * The AI attribute filter of the report bar: pick an attribute, then
 * the value it holds, or "Any value" for every conversation it was
 * found on. A choice offers its options and yes or no its two; other
 * types filter on being found. It shows as one chip, "Reason: billing",
 * that opens the same steps, with a cross to remove it.
 */
import { RiArrowLeftSLine, RiArrowRightSLine, RiCheckLine, RiCloseLine, RiListCheck3 } from "@remixicon/react";
import { useState } from "react";

import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { aiFilterSummary, aiValueOptions } from "@/lib/ai-values";
import type { ConversationAttribute } from "@/lib/api/conversation-attributes";
import { cx } from "@/utils/cx";

import type { ReportSearch } from "./search";

const row = cx(
  "flex min-h-9 w-full items-center gap-2.5 rounded-xl px-2 text-start text-body-medium text-text-primary",
  "outline-none hover:bg-background-primary-hover focus-visible:bg-background-primary-hover",
);

const slide = "motion-safe:animate-[filter-panel-in_180ms_ease-out]";

type Change = (changes: Partial<ReportSearch>) => void;

function BackHeader({ label, onBack }: { label: string; onBack?: () => void }) {
  return (
    <div className={cx("flex min-h-8 items-center", onBack ? "gap-0.5 ps-0.5" : "ps-2")}>
      {onBack ? (
        <button
          type="button"
          aria-label="Back"
          onClick={onBack}
          className="grid size-7 shrink-0 place-items-center rounded-lg text-foreground-icon-secondary outline-none hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-4 rtl:-scale-x-100" />
        </button>
      ) : null}
      <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{label}</span>
    </div>
  );
}

/*
 * AIAttributePanel walks from the attributes to one attribute's values.
 * Picking a value applies it and closes.
 */
export function AIAttributePanel({
  attributes,
  search,
  onChange,
  onDone,
  onBack,
}: {
  attributes: ConversationAttribute[];
  search: ReportSearch;
  onChange: Change;
  onDone: () => void;
  onBack?: () => void;
}) {
  const [picked, setPicked] = useState<string | null>(search.ai_attribute ?? null);
  const attribute = attributes.find((a) => a.key === picked);

  if (!attribute) {
    return (
      <div className={cx("flex flex-col gap-1", slide)}>
        <BackHeader label="AI attribute" onBack={onBack} />
        <div className="-mx-1 flex max-h-72 flex-col overflow-y-auto px-1">
          {attributes.map((a) => (
            <button key={a.key} type="button" onClick={() => setPicked(a.key)} className={row}>
              <span className="min-w-0 flex-1 truncate">{a.label}</span>
              <RiArrowRightSLine
                aria-hidden
                className="size-4 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
              />
            </button>
          ))}
        </div>
      </div>
    );
  }

  const current = search.ai_attribute === attribute.key ? (search.ai_attribute_value ?? "") : null;
  const choose = (value: string) => {
    onChange({ ai_attribute: attribute.key, ai_attribute_value: value || undefined });
    onDone();
  };
  const options = [{ id: "", label: "Any value" }, ...aiValueOptions(attribute)];

  return (
    <div key={attribute.key} className={cx("flex flex-col gap-1", slide)}>
      <BackHeader label={attribute.label} onBack={() => setPicked(null)} />
      <div
        role="listbox"
        aria-label={`${attribute.label} values`}
        className="-mx-1 flex max-h-72 flex-col overflow-y-auto px-1"
      >
        {options.map((o) => (
          <button
            key={o.id}
            type="button"
            role="option"
            aria-selected={current === o.id}
            onClick={() => choose(o.id)}
            className={row}
          >
            <span className="min-w-0 flex-1 truncate">{o.label}</span>
            {current === o.id ? (
              <RiCheckLine aria-hidden className="size-4 shrink-0 text-foreground-icon-primary" />
            ) : null}
          </button>
        ))}
      </div>
    </div>
  );
}

/*
 * AIAttributeChip is the filter in use: it opens the steps to change it,
 * and its cross removes it.
 */
export function AIAttributeChip({
  attributes,
  search,
  onChange,
}: {
  attributes: ConversationAttribute[];
  search: ReportSearch;
  onChange: Change;
}) {
  const [open, setOpen] = useState(false);
  if (!search.ai_attribute) {
    return null;
  }

  const summary = aiFilterSummary(attributes, search.ai_attribute, search.ai_attribute_value);
  const text = `${summary.attribute}: ${summary.value}`;

  return (
    <span className="inline-flex h-8 max-w-72 items-center rounded-full border border-border-button-default bg-background-secondary-default ps-1">
      <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
        <button
          type="button"
          aria-label={`Change filter: ${text}`}
          className="inline-flex h-full min-w-0 items-center gap-1.5 rounded-full ps-2 pe-1 text-body-2-medium text-text-primary outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiListCheck3 aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <span className="truncate">{text}</span>
        </button>
        <Popover aria-label="AI attribute" placement="bottom start" className="w-64">
          <AIAttributePanel attributes={attributes} search={search} onChange={onChange} onDone={() => setOpen(false)} />
        </Popover>
      </PopoverTrigger>
      <button
        type="button"
        aria-label={`Remove filter: ${text}`}
        onClick={() => onChange({ ai_attribute: undefined, ai_attribute_value: undefined })}
        className="me-1 grid size-6 shrink-0 place-items-center rounded-full text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCloseLine aria-hidden className="size-4" />
      </button>
    </span>
  );
}
