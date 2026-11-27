/*
 * Canonical URLs, Open Graph/Twitter metadata, and JSON-LD structured data
 * emitted for every prerendered page and kept in sync on client navigation.
 */
import { GITHUB_URL } from "./links";
import { BANDS, cents, FREE_CONVERSATIONS } from "./pricing/prices";
import { DEFAULT_TITLE, type Route } from "./routes";

export const SITE_URL = "https://trydimmi.com";
export const OG_IMAGE_URL = `${SITE_URL}/og.png`;
export const LOGO_URL = `${SITE_URL}/logo.png`;

const [FIRST_BAND] = BANDS;

export function canonicalUrl(path: string): string {
  const clean = path.split(/[?#]/)[0].replace(/\/+$/, "") || "/";
  return clean === "/" ? `${SITE_URL}/` : `${SITE_URL}${clean}`;
}

export function headElements(route: Route | undefined) {
  const title = route?.title ?? DEFAULT_TITLE;
  const description = route?.description ?? "";
  const url = canonicalUrl(route?.path ?? "/");

  return new Set([
    { type: "link", props: { rel: "canonical", href: url } },
    { type: "meta", props: { name: "description", content: description } },
    { type: "meta", props: { property: "og:type", content: "website" } },
    { type: "meta", props: { property: "og:site_name", content: "dimmi" } },
    { type: "meta", props: { property: "og:url", content: url } },
    { type: "meta", props: { property: "og:title", content: title } },
    { type: "meta", props: { property: "og:description", content: description } },
    { type: "meta", props: { property: "og:image", content: OG_IMAGE_URL } },
    { type: "meta", props: { property: "og:image:width", content: "1200" } },
    { type: "meta", props: { property: "og:image:height", content: "630" } },
    { type: "meta", props: { property: "og:image:alt", content: title } },
    { type: "meta", props: { name: "twitter:card", content: "summary_large_image" } },
    { type: "meta", props: { name: "twitter:title", content: title } },
    { type: "meta", props: { name: "twitter:description", content: description } },
    { type: "meta", props: { name: "twitter:image", content: OG_IMAGE_URL } },
  ]);
}

function upsertHeadTag(tag: "meta" | "link", matchAttr: string, matchVal: string, attrs: Record<string, string>) {
  let el = document.head.querySelector(`${tag}[${matchAttr}="${matchVal}"]`);
  if (!el) {
    el = document.createElement(tag);
    el.setAttribute(matchAttr, matchVal);
    document.head.appendChild(el);
  }
  for (const [k, v] of Object.entries(attrs)) {
    el.setAttribute(k, v);
  }
}

/* Keeps document.head in sync during client-side swaps and Vite dev mode. */
export function syncHead(route: Route | undefined) {
  const title = route?.title ?? DEFAULT_TITLE;
  const description = route?.description ?? "";
  const url = canonicalUrl(route?.path ?? "/");

  document.title = title;
  upsertHeadTag("link", "rel", "canonical", { href: url });
  upsertHeadTag("meta", "name", "description", { content: description });
  upsertHeadTag("meta", "property", "og:type", { content: "website" });
  upsertHeadTag("meta", "property", "og:site_name", { content: "dimmi" });
  upsertHeadTag("meta", "property", "og:url", { content: url });
  upsertHeadTag("meta", "property", "og:title", { content: title });
  upsertHeadTag("meta", "property", "og:description", { content: description });
  upsertHeadTag("meta", "property", "og:image", { content: OG_IMAGE_URL });
  upsertHeadTag("meta", "property", "og:image:width", { content: "1200" });
  upsertHeadTag("meta", "property", "og:image:height", { content: "630" });
  upsertHeadTag("meta", "property", "og:image:alt", { content: title });
  upsertHeadTag("meta", "name", "twitter:card", { content: "summary_large_image" });
  upsertHeadTag("meta", "name", "twitter:title", { content: title });
  upsertHeadTag("meta", "name", "twitter:description", { content: description });
  upsertHeadTag("meta", "name", "twitter:image", { content: OG_IMAGE_URL });
}

const PAID_CLOUD_OFFER = {
  "@type": "Offer",
  name: "dimmi Cloud",
  price: (FIRST_BAND.human / 100).toFixed(2),
  priceCurrency: "USD",
  description: `First ${FREE_CONVERSATIONS} conversations free each month, then from ${cents(FIRST_BAND.ai)} per AI conversation and ${cents(FIRST_BAND.human)} per human conversation with unlimited free seats.`,
  priceSpecification: [
    {
      "@type": "UnitPriceSpecification",
      name: "AI conversation",
      price: (FIRST_BAND.ai / 100).toFixed(2),
      priceCurrency: "USD",
      unitText: "conversation",
    },
    {
      "@type": "UnitPriceSpecification",
      name: "Human conversation",
      price: (FIRST_BAND.human / 100).toFixed(2),
      priceCurrency: "USD",
      unitText: "conversation",
    },
  ],
};

const SOFTWARE_SCHEMA = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "dimmi",
  applicationCategory: "BusinessApplication",
  operatingSystem: "Web, Linux",
  url: `${SITE_URL}/`,
  license: "https://opensource.org/licenses/MIT",
  description:
    "Open-source customer support platform with a shared inbox, specialized AI support agents, procedures, Operator, tickets, workflows, and help center.",
  offers: [
    {
      "@type": "Offer",
      name: "Self-hosted (MIT)",
      price: "0",
      priceCurrency: "USD",
      description: "MIT open-source build on your own Linux server with every feature included.",
    },
    {
      "@type": "Offer",
      name: "dimmi Cloud Free Tier",
      price: "0",
      priceCurrency: "USD",
      description: `${FREE_CONVERSATIONS} free conversations every month with unlimited free seats and every feature unlocked.`,
    },
    PAID_CLOUD_OFFER,
  ],
};

const ORGANIZATION_SCHEMA = {
  "@context": "https://schema.org",
  "@type": "Organization",
  name: "dimmi",
  legalName: "Hivy Technologies Limited",
  url: `${SITE_URL}/`,
  logo: {
    "@type": "ImageObject",
    url: LOGO_URL,
    width: 512,
    height: 512,
  },
  // Add confirmed X and LinkedIn profile URLs once provided.
  sameAs: [GITHUB_URL],
};

const WEBSITE_SCHEMA = {
  "@context": "https://schema.org",
  "@type": "WebSite",
  name: "dimmi",
  url: `${SITE_URL}/`,
};

function breadcrumbLabel(route: Route): string {
  const beforeColon = route.title.split(":")[0]?.trim();
  const beforePipe = beforeColon?.split("|")[0]?.trim();
  return beforePipe || route.path;
}

export function pageSchemas(route: Route | undefined): Record<string, unknown>[] {
  if (!route) {
    return [];
  }
  if (route.path === "/") {
    return [ORGANIZATION_SCHEMA, WEBSITE_SCHEMA, SOFTWARE_SCHEMA];
  }

  const items: { "@type": "ListItem"; position: number; name: string; item: string }[] = [
    { "@type": "ListItem", position: 1, name: "Home", item: `${SITE_URL}/` },
  ];

  if (route.path.startsWith("/channels/")) {
    items.push({ "@type": "ListItem", position: 2, name: "Channels", item: `${SITE_URL}/channels` });
    items.push({ "@type": "ListItem", position: 3, name: breadcrumbLabel(route), item: canonicalUrl(route.path) });
  } else {
    items.push({ "@type": "ListItem", position: 2, name: breadcrumbLabel(route), item: canonicalUrl(route.path) });
  }

  const schemas: Record<string, unknown>[] = [
    {
      "@context": "https://schema.org",
      "@type": "BreadcrumbList",
      itemListElement: items,
    },
  ];

  if (route.path === "/pricing" || route.path === "/self-host") {
    schemas.push(SOFTWARE_SCHEMA);
  }

  if (route.meta) {
    schemas.push({
      "@context": "https://schema.org",
      "@type": "WebPage",
      name: route.title,
      description: route.description,
      url: canonicalUrl(route.path),
      datePublished: route.meta.published,
      dateModified: route.meta.updated,
      author: { "@type": "Person", name: "Frantz Kati" },
      reviewedBy: { "@type": "Person", name: "Frantz Kati" },
    });
  }

  return schemas;
}
