import { DimmiLogo } from "@/features/brand/logo";
import { GITHUB_URL } from "../links";
import { PRODUCT_GROUPS } from "../products";

// Links that have a page; the rest of the footer is still plain text.
const pages = new Map(PRODUCT_GROUPS.flatMap((group) => group.products.map((p) => [p.name, p.href])));

const columns = [
  {
    title: "Product",
    links: PRODUCT_GROUPS.flatMap((group) => group.products.map((p) => p.name)),
  },
  {
    title: "Channels",
    links: ["Messenger", "Email", "Slack", "Microsoft Teams", "Discord", "WhatsApp & SMS", "Phone", "Headless & API"],
  },
  { title: "Run it", links: ["Self-host", "dimmi Cloud", "Security", "Migrate to dimmi"] },
  { title: "Developers", links: ["Docs", "API reference", "SDKs", "Webhooks", "GitHub"] },
  { title: "Company", links: ["Pricing", "About", "Careers", "Contact", "Privacy", "Terms"] },
];

export function SiteFooter() {
  return (
    <footer className="dark site-brand border-t border-separator-border bg-background-full">
      <div className="mx-auto w-full max-w-[1200px] border-x border-separator-border px-5 sm:px-12 grid grid-cols-2 gap-x-8 gap-y-12 py-16 sm:grid-cols-3 lg:grid-cols-[1.4fr_repeat(5,1fr)] [&>*]:min-w-0">
        <div className="col-span-full flex flex-col gap-4 lg:col-span-1">
          <DimmiLogo className="h-6 w-auto self-start text-text-primary" title="dimmi" />
          <p className="max-w-60 text-body-regular text-text-secondary">
            Open-source customer support. MIT licensed, on our cloud or your servers.
          </p>
          <a href={GITHUB_URL} className="text-body-medium text-text-primary underline-offset-4 hover:underline">
            github.com/dimmihq/dimmi
          </a>
        </div>
        {columns.map((column) => (
          <div key={column.title} className="flex flex-col gap-3">
            <h3 className="font-mono text-[11px] tracking-[0.14em] text-text-tertiary uppercase">{column.title}</h3>
            <ul className="flex flex-col gap-2">
              {column.links.map((link) => (
                <li key={link}>
                  {pages.has(link) ? (
                    <a
                      href={pages.get(link)}
                      className="text-body-regular text-text-secondary transition-colors hover:text-text-primary"
                    >
                      {link}
                    </a>
                  ) : (
                    <span className="text-body-regular text-text-secondary">{link}</span>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="mx-auto w-full max-w-[1200px] border-x border-separator-border px-5 sm:px-12 flex flex-wrap items-center justify-between gap-4 border-t border-separator-border py-6 text-caption-1-regular text-text-tertiary">
        <span>© 2026 dimmi. MIT licensed.</span>
      </div>
    </footer>
  );
}
