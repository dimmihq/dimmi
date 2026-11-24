import { DimmiLogo } from "@/features/brand/logo";
import { LEGAL_ENTITY } from "../legal/constants";
import { GITHUB_URL } from "../links";
import { PRODUCT_GROUPS } from "../products";

// Links that have a page; the rest of the footer is still plain text.
const pages = new Map<string, string>([
  ...PRODUCT_GROUPS.flatMap((group) => group.products.map((p) => [p.name, p.href] as const)),
  ["Pricing", "/pricing"],
  ["Compare", "/compare"],
  ["Self-host", "/#self-host"],
  ["Security", "/privacy#security-overview"],
  ["Terms", "/terms"],
  ["Acceptable Use", "/terms#acceptable-use"],
  ["Self-Hosting Notice", "/terms#self-hosting-notice"],
  ["Privacy", "/privacy"],
  ["DPA", "/privacy#dpa"],
  ["Subprocessors", "/privacy#subprocessors"],
  ["Cookies", "/privacy#cookie-notice"],
  ["AI Transparency", "/privacy#ai-transparency"],
]);

const columns = [
  {
    title: "Product",
    links: PRODUCT_GROUPS.flatMap((group) => group.products.map((p) => p.name)),
  },
  {
    title: "Channels",
    links: ["Messenger", "Email", "Slack", "Microsoft Teams", "Discord", "WhatsApp & SMS", "Phone", "Headless & API"],
  },
  { title: "Run it", links: ["Self-host", "dimmi Cloud", "Security", "Self-Hosting Notice", "AI Transparency"] },
  { title: "Developers", links: ["Docs", "API reference", "SDKs", "Webhooks", "GitHub"] },
  {
    title: "Company & Legal",
    links: ["Pricing", "Compare", "Terms", "Acceptable Use", "Privacy", "DPA", "Subprocessors", "Cookies"],
  },
];

export function SiteFooter() {
  return (
    <footer className="dark site-brand border-t border-separator-border bg-background-full">
      <div className="mx-auto w-full max-w-[1200px] border-x border-separator-border px-5 sm:px-12 grid grid-cols-2 gap-x-8 gap-y-12 py-16 sm:grid-cols-3 lg:grid-cols-[1.4fr_repeat(5,1fr)] [&>*]:min-w-0">
        <div className="col-span-full flex flex-col gap-4 lg:col-span-1">
          <DimmiLogo className="h-6 w-auto self-start text-text-primary" title="dimmi" />
          <p className="max-w-60 text-body-regular text-text-secondary">
            Customer support you own. Open source under MIT, on our cloud or your own server.
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
        <span>
          © 2026 {LEGAL_ENTITY.name} ({LEGAL_ENTITY.rcNumber}) · {LEGAL_ENTITY.address} · dimmi is MIT licensed.
        </span>
        <div className="flex flex-wrap items-center gap-4">
          <a href="/privacy" className="hover:text-text-primary">
            Privacy Policy
          </a>
          <a href="/privacy#cookie-notice" className="hover:text-text-primary">
            Cookie Notice
          </a>
          <a href="/terms" className="hover:text-text-primary">
            Terms of Service
          </a>
          <a href="/privacy#dpa" className="hover:text-text-primary">
            DPA
          </a>
        </div>
      </div>
    </footer>
  );
}
