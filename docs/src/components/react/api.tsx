import { RiArrowDownSLine, RiArrowRightLine } from "@remixicon/react";
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

export function ApiSection({
  id,
  title,
  meta,
  children,
}: {
  id?: string;
  title: string;
  meta?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col">
      <div className="flex items-baseline justify-between gap-3 border-b border-separator-border pb-3">
        <h2 id={id} className="scroll-mt-[calc(var(--docs-header)+2rem)] text-headline-semibold text-text-primary">
          {title}
        </h2>
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

export type RelatedGuideLink = { title: string; href: string; description: string };
export type RelatedCategoryLink = { label: string; title: string; href: string; count: number };
export type RelatedEndpointLink = { title: string; href: string; method: string; path: string };

export function ApiRelated({
  guide,
  category,
  siblings,
}: {
  guide: RelatedGuideLink;
  category: RelatedCategoryLink;
  siblings: RelatedEndpointLink[];
}) {
  return (
    <ApiSection title="Related">
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <a
          href={guide.href}
          className="group flex min-w-0 flex-col justify-between gap-2 rounded-2xl border border-border-button-default bg-background-full p-4 shadow-xs transition-colors hover:border-border-button-hover"
        >
          <span className="flex items-center justify-between gap-2">
            <span className="text-caption-1-medium text-accent-600 dark:text-accent-400">Guide</span>
            <RiArrowRightLine
              className="size-4 shrink-0 text-foreground-icon-tertiary transition-transform group-hover:translate-x-0.5"
              aria-hidden
            />
          </span>
          <span className="flex min-w-0 flex-col gap-1">
            <span className="text-body-medium text-text-primary">{guide.title}</span>
            <span className="text-caption-1-regular text-text-secondary">{guide.description}</span>
          </span>
        </a>
        <a
          href={category.href}
          className="group flex min-w-0 flex-col justify-between gap-2 rounded-2xl border border-border-button-default bg-background-full p-4 shadow-xs transition-colors hover:border-border-button-hover"
        >
          <span className="flex items-center justify-between gap-2">
            <span className="text-caption-1-medium text-text-tertiary">API category</span>
            <RiArrowRightLine
              className="size-4 shrink-0 text-foreground-icon-tertiary transition-transform group-hover:translate-x-0.5"
              aria-hidden
            />
          </span>
          <span className="flex min-w-0 flex-col gap-1">
            <span className="text-body-medium text-text-primary">{category.title}</span>
            <span className="text-caption-1-regular text-text-secondary">
              Browse all {category.count} {category.count === 1 ? "endpoint" : "endpoints"} in {category.label}.
            </span>
          </span>
        </a>
        {siblings.map((endpoint) => (
          <a
            key={endpoint.href}
            href={endpoint.href}
            className="group flex min-w-0 flex-col justify-between gap-2 rounded-2xl border border-border-button-default bg-background-full p-4 shadow-xs transition-colors hover:border-border-button-hover"
          >
            <span className="flex min-w-0 items-center justify-between gap-2">
              <span className="flex min-w-0 items-center gap-2">
                <MethodChip method={endpoint.method} />
                <code className="truncate font-mono text-caption-1-regular text-text-tertiary">{endpoint.path}</code>
              </span>
              <RiArrowRightLine
                className="size-4 shrink-0 text-foreground-icon-tertiary transition-transform group-hover:translate-x-0.5"
                aria-hidden
              />
            </span>
            <span className="text-body-medium text-text-primary">{endpoint.title}</span>
          </a>
        ))}
      </div>
    </ApiSection>
  );
}

export type CategoryEndpointRow = {
  title: string;
  href: string;
  method: string;
  path: string;
  description: string;
};

export function ApiCategoryList({ endpoints }: { endpoints: CategoryEndpointRow[] }) {
  return (
    <div className="flex flex-col divide-y divide-separator-border">
      {endpoints.map((endpoint) => (
        <a
          key={endpoint.href}
          href={endpoint.href}
          className="group flex min-w-0 flex-col gap-1.5 py-4 transition-colors sm:flex-row sm:items-start sm:justify-between sm:gap-4"
        >
          <span className="flex min-w-0 flex-1 flex-col gap-1.5">
            <span className="flex min-w-0 flex-wrap items-center gap-2">
              <MethodChip method={endpoint.method} />
              <span className="text-body-medium text-text-primary group-hover:text-accent-700 dark:group-hover:text-accent-400">
                {endpoint.title}
              </span>
              <code className="max-w-full truncate font-mono text-caption-1-regular text-text-tertiary">
                {endpoint.path}
              </code>
            </span>
            <span className="text-body-regular text-text-secondary">{endpoint.description}</span>
          </span>
          <RiArrowRightLine
            className="hidden size-4 shrink-0 text-foreground-icon-tertiary transition-transform group-hover:translate-x-0.5 sm:mt-1 sm:block"
            aria-hidden
          />
        </a>
      ))}
    </div>
  );
}
