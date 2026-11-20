import { RiArrowDownSLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import type { Field } from "../../api/spec";
import { MethodChip } from "./content";

/*
 * The parts of an API reference page, rendered to static HTML: the
 * endpoint line, titled sections and the field lists. Nested objects fold
 * open under "Show child attributes" on BoardUI's disclosure motion.
 */

export function EndpointBar({ method, path, base }: { method: string; path: string; base: string }) {
  const parts = path.split(/(\{[^}]+\})/).filter(Boolean);
  return (
    <div className="flex items-center gap-3 overflow-x-auto rounded-2xl border border-separator-border bg-background-secondary-default px-3 py-2.5 [scrollbar-width:none]">
      <MethodChip method={method} />
      <code className="font-mono text-[0.8125rem] whitespace-nowrap">
        <span className="text-text-tertiary">{base}</span>
        {parts.map((part, index) => (
          <span
            // biome-ignore lint/suspicious/noArrayIndexKey: path pieces repeat and never reorder
            key={index}
            className={part.startsWith("{") ? "text-accent-700 dark:text-accent-400" : "text-text-primary"}
          >
            {part}
          </span>
        ))}
      </code>
    </div>
  );
}

export function ApiSection({ title, meta, children }: { title: string; meta?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col">
      <div className="flex items-baseline justify-between gap-3 border-b border-separator-border pb-3">
        <h2 className="text-headline-semibold text-text-primary">{title}</h2>
        {meta ? <span className="font-mono text-caption-1-regular text-text-tertiary">{meta}</span> : null}
      </div>
      {children}
    </section>
  );
}

export function ApiFields({ fields, nested = false }: { fields: Field[]; nested?: boolean }) {
  return (
    <div className={cx("flex flex-col", nested && "rounded-2xl border border-separator-border px-4")}>
      {fields.map((field) => (
        <FieldRow key={field.name} field={field} />
      ))}
    </div>
  );
}

function FieldRow({ field }: { field: Field }) {
  return (
    <div className="docs-field">
      <div className="docs-field-head">
        <code className="docs-field-name">{field.name}</code>
        <span className="docs-field-type">{field.type}</span>
        {field.required ? (
          <Chip variant="caption" color="rose">
            required
          </Chip>
        ) : null}
        {field.defaultValue ? (
          <span className="text-caption-1-regular text-text-tertiary">
            default <code className="font-mono text-text-secondary">{field.defaultValue}</code>
          </span>
        ) : null}
      </div>
      {field.description ? <p className="docs-field-text">{field.description}</p> : null}
      {field.options?.length ? (
        <p className="flex flex-wrap items-center gap-1.5 text-caption-1-regular text-text-tertiary">
          Available options:
          {field.options.map((option) => (
            <code
              key={option}
              className="rounded-md bg-background-secondary-default px-1.5 py-0.5 font-mono text-text-secondary"
            >
              {option}
            </code>
          ))}
        </p>
      ) : null}
      {field.notes.length ? <p className="docs-field-note">{field.notes.join(" · ")}</p> : null}
      {field.children.length ? <Children fields={field.children} /> : null}
    </div>
  );
}

function Children({ fields }: { fields: Field[] }) {
  return (
    <details className="docs-disclosure group/child">
      <summary className="inline-flex cursor-pointer list-none items-center gap-1 rounded-lg py-0.5 text-caption-1-medium text-text-secondary transition-colors hover:text-text-primary [&::-webkit-details-marker]:hidden">
        <RiArrowDownSLine
          aria-hidden
          className="size-4 -rotate-90 transition-[rotate] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] group-open/child:rotate-0 motion-reduce:transition-none"
        />
        <span className="group-open/child:hidden">Show child attributes</span>
        <span className="hidden group-open/child:inline">Hide child attributes</span>
      </summary>
      <div className="docs-disclosure-body pt-2">
        <ApiFields fields={fields} nested />
      </div>
    </details>
  );
}
