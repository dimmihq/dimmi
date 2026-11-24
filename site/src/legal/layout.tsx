/*
 * Editorial layout and typography primitives for dimmi's two comprehensive
 * legal documents (/terms and /privacy). Built inside the site's 1200px
 * hairline-rail frame with a subtle grid hero, two-page switcher, Part jump
 * cards, grouped sticky navigation, and inline accent-bordered plain-English
 * summaries placed naturally beneath each clause heading.
 */
import { RiArrowRightUpLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { cx } from "@/utils/cx";
import { LEGAL_CONTACTS, LEGAL_ENTITY, LEGAL_VERSION } from "./constants";

export type TocItem = {
  id: string;
  label: string;
};

export type PartSummary = {
  id: string;
  numeral: string;
  title: string;
  description: string;
  items: TocItem[];
};

const DOC_TABS = [
  { href: "/terms", label: "Terms of Service" },
  { href: "/privacy", label: "Privacy Policy & DPA" },
] as const;

export function LegalDocument({
  path,
  title,
  subtitle,
  parts,
  children,
}: {
  path: "/terms" | "/privacy";
  title: string;
  subtitle: string;
  parts: PartSummary[];
  children: ReactNode;
}) {
  return (
    <div className="bg-background-full">
      {/* Editorial hero with dimmi's subtle radial-masked hairline grid */}
      <section className="relative border-b border-separator-border bg-background-full">
        <div className="relative mx-auto w-full max-w-[1200px] overflow-hidden border-x border-separator-border">
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 opacity-55 [background-image:linear-gradient(var(--color-separator-border)_1px,transparent_1px),linear-gradient(90deg,var(--color-separator-border)_1px,transparent_1px)] [background-size:64px_64px] [mask-image:radial-gradient(65%_55%_at_50%_20%,black,transparent)]"
          />
          <div className="relative flex flex-col gap-7 px-5 pt-16 pb-12 sm:px-12 md:pt-24 md:pb-16">
            {/* Two-document segmented switcher */}
            <nav
              aria-label="Legal documents"
              className="inline-flex self-start rounded-xl border border-separator-border bg-background-secondary-default/75 p-1 backdrop-blur-sm"
            >
              {DOC_TABS.map((tab) => {
                const active = tab.href === path;
                return (
                  <a
                    key={tab.href}
                    href={tab.href}
                    aria-current={active ? "page" : undefined}
                    className={cx(
                      "rounded-lg px-4 py-1.5 text-[14px] leading-5 font-medium transition-colors",
                      active
                        ? "bg-background-full text-text-primary shadow-xs"
                        : "text-text-secondary hover:text-text-primary",
                    )}
                  >
                    {tab.label}
                  </a>
                );
              })}
            </nav>

            <div className="flex max-w-3xl flex-col gap-4">
              <h1 className="text-[48px] leading-[1.06] font-medium tracking-[-0.032em] text-balance text-text-primary max-md:text-[38px] max-sm:text-[32px]">
                {title}
              </h1>
              <p className="max-w-2xl text-[17px] leading-7 text-pretty text-text-secondary">{subtitle}</p>
            </div>

            {/* Crisp statutory metadata bar */}
            <dl className="flex flex-wrap items-center gap-x-8 gap-y-3 border-t border-separator-border pt-6 text-[13px] leading-5">
              <div className="flex items-center gap-2">
                <dt className="font-mono text-[11px] tracking-[0.08em] text-text-tertiary uppercase">Version</dt>
                <dd className="rounded-md bg-background-secondary-default px-2 py-0.5 font-mono text-[12px] font-medium text-text-primary">
                  v{LEGAL_VERSION.version}
                </dd>
              </div>
              <div className="flex items-center gap-2">
                <dt className="font-mono text-[11px] tracking-[0.08em] text-text-tertiary uppercase">Published</dt>
                <dd className="text-text-primary">{LEGAL_VERSION.publishedDate}</dd>
              </div>
              <div className="flex items-center gap-2">
                <dt className="font-mono text-[11px] tracking-[0.08em] text-text-tertiary uppercase">Effective</dt>
                <dd className="font-medium text-text-primary">{LEGAL_VERSION.effectiveDate}</dd>
              </div>
              <div className="flex items-center gap-2">
                <dt className="font-mono text-[11px] tracking-[0.08em] text-text-tertiary uppercase">Entity</dt>
                <dd className="text-text-primary">
                  {LEGAL_ENTITY.name} ({LEGAL_ENTITY.rcNumber})
                </dd>
              </div>
            </dl>
          </div>

          {/* Part quick-jump grid right beneath the hero */}
          <div className="relative border-t border-separator-border bg-background-secondary-default/35">
            <div
              className={cx(
                "grid grid-cols-1 divide-y divide-separator-border sm:grid-cols-2 sm:divide-y-0",
                parts.length <= 4 ? "lg:grid-cols-4" : "lg:grid-cols-4",
              )}
            >
              {parts.map((part) => (
                <a
                  key={part.id}
                  href={`#${part.id}`}
                  className="group flex flex-col gap-1.5 border-separator-border p-5 transition-colors hover:bg-background-full sm:border-e sm:border-b lg:px-6"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-[11px] font-medium tracking-[0.1em] text-accent-600 uppercase dark:text-accent-400">
                      Part {part.numeral}
                    </span>
                    <RiArrowRightUpLine
                      className="size-3.5 text-text-tertiary transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-text-primary"
                      aria-hidden
                    />
                  </div>
                  <span className="text-[15px] leading-5 font-medium text-text-primary">{part.title}</span>
                  <span className="text-[13px] leading-5 text-text-tertiary">{part.description}</span>
                </a>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Main two-column editorial body */}
      <section className="relative bg-background-full">
        <div className="mx-auto grid w-full max-w-[1200px] grid-cols-1 border-x border-separator-border lg:grid-cols-[272px_minmax(0,1fr)]">
          <aside className="border-b border-separator-border px-5 py-8 sm:px-8 lg:border-e lg:border-b-0 lg:py-12">
            <div className="flex flex-col gap-6 lg:sticky lg:top-24 lg:max-h-[calc(100vh-8rem)] lg:overflow-y-auto lg:pe-3">
              <nav aria-label="Document sections" className="flex flex-col gap-6">
                {parts.map((part) => (
                  <div key={part.id} className="flex flex-col gap-2">
                    <a
                      href={`#${part.id}`}
                      className="flex items-baseline gap-2 font-mono text-[11px] font-medium tracking-[0.1em] text-text-primary uppercase transition-colors hover:text-accent-600 dark:hover:text-accent-400"
                    >
                      <span className="text-text-tertiary">{part.numeral}.</span>
                      <span>{part.title}</span>
                    </a>
                    <ol className="flex flex-col gap-1 border-s border-separator-border ps-3 text-[13px] leading-5">
                      {part.items.map((item) => (
                        <li key={item.id}>
                          <a
                            href={`#${item.id}`}
                            className="block py-0.5 text-text-secondary transition-colors hover:text-text-primary"
                          >
                            {item.label}
                          </a>
                        </li>
                      ))}
                    </ol>
                  </div>
                ))}
              </nav>

              <div className="border-t border-separator-border pt-5 text-[12px] leading-5 text-text-tertiary">
                <p className="font-mono text-[10px] font-medium tracking-[0.1em] text-text-tertiary uppercase">
                  Official contacts
                </p>
                <p className="mt-1.5">
                  Legal, privacy, DPA, security &amp; abuse:{" "}
                  <a
                    href={`mailto:${LEGAL_CONTACTS.legal}`}
                    className="font-medium text-text-primary underline decoration-separator-border-strong underline-offset-2 hover:decoration-text-primary"
                  >
                    {LEGAL_CONTACTS.legal}
                  </a>
                </p>
                <p className="mt-1">
                  Support &amp; billing:{" "}
                  <a
                    href={`mailto:${LEGAL_CONTACTS.support}`}
                    className="font-medium text-text-primary underline decoration-separator-border-strong underline-offset-2 hover:decoration-text-primary"
                  >
                    {LEGAL_CONTACTS.support}
                  </a>
                </p>
              </div>
            </div>
          </aside>

          {/* Spacious, single-column editorial reading area */}
          <article className="min-w-0 px-5 py-10 sm:px-10 md:px-14 md:py-16">
            <div className="mx-auto flex max-w-[740px] flex-col gap-12">{children}</div>
          </article>
        </div>
      </section>
    </div>
  );
}

/*
 * Editorial divider opening each major Part inside /terms or /privacy.
 */
export function PartHeader({
  anchor,
  numeral,
  title,
  binding,
  subtitle,
}: {
  anchor: string;
  numeral: string;
  title: string;
  binding: string;
  subtitle: string;
}) {
  return (
    <header
      id={anchor}
      className="scroll-mt-24 rounded-2xl border border-separator-border bg-background-secondary-default/50 px-6 py-6 sm:px-7 sm:py-7"
    >
      <div className="flex flex-wrap items-center gap-3">
        <span className="inline-flex items-center gap-2 font-mono text-[11px] font-medium tracking-[0.12em] text-accent-700 uppercase dark:text-accent-300">
          <span className="size-1.5 rounded-full bg-accent-500" aria-hidden />
          Part {numeral}
        </span>
        <span className="text-text-tertiary" aria-hidden>
          ·
        </span>
        <span className="font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">{binding}</span>
      </div>
      <h2 className="mt-2.5 text-[28px] leading-[1.18] font-medium tracking-[-0.025em] text-text-primary max-sm:text-[24px]">
        {title}
      </h2>
      <p className="mt-2 text-[15px] leading-6 text-pretty text-text-secondary">{subtitle}</p>
    </header>
  );
}

/*
 * A numbered clause in clean editorial single-column flow. When a plain-English
 * summary is provided, it sits directly under the heading with a refined
 * vertical accent rule so the reader gets the takeaway before the legal text
 * without cramped side-column boxes.
 */
export function Clause({
  anchor,
  number,
  title,
  summary,
  children,
}: {
  anchor: string;
  number: string;
  title: string;
  summary?: string;
  children: ReactNode;
}) {
  return (
    <section id={anchor} className="scroll-mt-24 border-t border-separator-border pt-9 first:border-t-0 first:pt-0">
      <div className="mb-4 flex items-baseline gap-3">
        <span className="font-mono text-[12px] font-medium tracking-[0.04em] text-text-tertiary">{number}</span>
        <h3 className="text-[21px] leading-7 font-medium tracking-[-0.02em] text-text-primary">{title}</h3>
      </div>

      {summary ? (
        <div className="mb-5 border-s-2 border-accent-500/60 ps-4 py-0.5">
          <span className="block font-mono text-[10px] font-medium tracking-[0.1em] text-accent-700 uppercase dark:text-accent-300">
            In plain English (non-binding)
          </span>
          <p className="mt-1 text-[14px] leading-6 text-pretty text-text-primary/85">{summary}</p>
        </div>
      ) : null}

      <div className="flex min-w-0 flex-col gap-4 text-[15px] leading-[1.75] text-text-secondary">{children}</div>
    </section>
  );
}

export function SubHeading({ children }: { children: ReactNode }) {
  return <h4 className="mt-3 text-[16px] leading-6 font-medium text-text-primary">{children}</h4>;
}

export function NoticeBox({
  title,
  tone = "neutral",
  children,
}: {
  title: string;
  tone?: "neutral" | "emphasis";
  children: ReactNode;
}) {
  return (
    <div
      className={cx(
        "rounded-xl border px-5 py-5 sm:px-6",
        tone === "emphasis"
          ? "border-accent-500/35 bg-accent-500/[0.05]"
          : "border-separator-border bg-background-secondary-default/45",
      )}
    >
      <p className="mb-2.5 font-mono text-[11px] font-medium tracking-[0.08em] text-text-primary uppercase">{title}</p>
      <div className="flex flex-col gap-3 text-[14px] leading-6 text-text-secondary">{children}</div>
    </div>
  );
}

export function LegalList({ items }: { items: ReactNode[] }) {
  return (
    <ul className="flex list-disc flex-col gap-2.5 ps-5 text-[15px] leading-[1.7] text-text-secondary marker:text-text-tertiary">
      {items.map((item, idx) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: static legal list items
        <li key={idx}>{item}</li>
      ))}
    </ul>
  );
}

export function LegalTable({ headers, rows, caption }: { headers: string[]; rows: ReactNode[][]; caption?: string }) {
  return (
    <div className="my-1 overflow-x-auto rounded-xl border border-separator-border">
      <table className="w-full border-collapse text-left text-[14px] leading-6">
        {caption ? <caption className="sr-only">{caption}</caption> : null}
        <thead>
          <tr className="border-b border-separator-border bg-background-secondary-default/65">
            {headers.map((header) => (
              <th
                key={header}
                scope="col"
                className="px-4 py-3 font-mono text-[11px] font-medium tracking-[0.06em] text-text-tertiary uppercase"
              >
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-separator-border bg-background-full">
          {rows.map((row, rowIdx) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static legal table rows
            <tr key={rowIdx} className="align-top">
              {row.map((cell, cellIdx) => (
                <td
                  key={headers[cellIdx] ?? `col-${cellIdx}`}
                  className={cx("px-4 py-3 text-text-secondary", cellIdx === 0 && "font-medium text-text-primary")}
                >
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function LegalLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a
      href={href}
      className="font-medium text-text-primary underline decoration-separator-border-strong underline-offset-4 hover:decoration-text-primary"
    >
      {children}
    </a>
  );
}

export function LegalEntityFooter() {
  return (
    <footer className="mt-4 rounded-xl border border-separator-border bg-background-secondary-default/35 p-5 text-[13px] leading-6 text-text-tertiary">
      <p className="font-medium text-text-primary">
        {LEGAL_ENTITY.name} ({LEGAL_ENTITY.rcNumber})
      </p>
      <p className="mt-1">
        Incorporated in the {LEGAL_ENTITY.country} on {LEGAL_ENTITY.incorporationDate} under the{" "}
        {LEGAL_ENTITY.incorporationAct}. Registered office: {LEGAL_ENTITY.address}.
      </p>
      <p className="mt-2">
        Version v{LEGAL_VERSION.version} · Published {LEGAL_VERSION.publishedDate} · Effective{" "}
        {LEGAL_VERSION.effectiveDate}. Legal, privacy, security &amp; abuse:{" "}
        <LegalLink href={`mailto:${LEGAL_CONTACTS.legal}`}>{LEGAL_CONTACTS.legal}</LegalLink> · Support &amp; billing:{" "}
        <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>.
      </p>
    </footer>
  );
}
