import { existsSync, statSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { type PageMeta, type PageType, type Route, routes } from "./routes";

const SITE_DIR = resolve(process.cwd(), "site");
const ALLOWED_REVIEWERS = new Set(["frantz-kati"]);
const VALID_PAGE_TYPES = new Set<PageType>([
  "marketing",
  "vs",
  "alternatives",
  "blog",
  "self-host",
  "tool",
  "docs",
  "api-reference",
]);
const ISO_DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

function isValidIsoDate(value: string): boolean {
  const match = ISO_DATE_RE.exec(value);
  if (!match) {
    return false;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
}

function isHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

function proofExists(siteDir: string, proofPath: string): boolean {
  const clean = proofPath.trim().replace(/^\/+/, "");
  if (!clean) {
    return false;
  }
  const candidates = [resolve(siteDir, "public", clean), resolve(siteDir, clean)];
  return candidates.some((candidate) => existsSync(candidate) && statSync(candidate).isFile());
}

function validateMetaRivals(path: string, meta: PageMeta, errors: string[]): void {
  if (!meta.rivals) {
    return;
  }
  for (const rival of meta.rivals) {
    if (!rival.name.trim()) {
      errors.push(`${path}: meta.rivals entry has empty name`);
    }
    if (!isValidIsoDate(rival.checkedOn)) {
      errors.push(`${path}: meta.rivals (${rival.name}) has invalid checkedOn "${rival.checkedOn}"`);
    }
    if (!isHttpUrl(rival.source)) {
      errors.push(`${path}: meta.rivals (${rival.name}) has invalid source URL "${rival.source}"`);
    }
  }
}

function validateMetaProof(path: string, meta: PageMeta, siteDir: string, errors: string[]): void {
  if (!meta.proof) {
    return;
  }
  for (const item of meta.proof) {
    if (!proofExists(siteDir, item)) {
      errors.push(`${path}: meta.proof file not found "${item}"`);
    }
  }
}

function validateMetaDates(path: string, meta: PageMeta, errors: string[]): void {
  for (const [label, value] of [
    ["published", meta.published],
    ["updated", meta.updated],
    ["reviewedOn", meta.reviewedOn],
  ] as const) {
    if (!isValidIsoDate(value)) {
      errors.push(`${path}: meta.${label} must be a valid YYYY-MM-DD date, got "${value}"`);
    }
  }
  if (isValidIsoDate(meta.published) && isValidIsoDate(meta.updated) && meta.updated < meta.published) {
    errors.push(`${path}: meta.updated (${meta.updated}) is earlier than meta.published (${meta.published})`);
  }
}

function validateMeta(path: string, meta: PageMeta, siteDir: string, errors: string[]): void {
  if (!VALID_PAGE_TYPES.has(meta.type)) {
    errors.push(`${path}: invalid meta.type "${meta.type}"`);
  }
  if (!meta.keyword.trim()) {
    errors.push(`${path}: meta.keyword must be non-empty`);
  }
  if (!meta.answer.trim()) {
    errors.push(`${path}: meta.answer must be non-empty`);
  }
  if (!meta.author.trim()) {
    errors.push(`${path}: meta.author must be non-empty`);
  }
  if (meta.status !== "draft" && meta.status !== "published") {
    errors.push(`${path}: meta.status must be "draft" or "published"`);
  }
  if (meta.status === "published" && !ALLOWED_REVIEWERS.has(meta.reviewedBy)) {
    errors.push(`${path}: meta.reviewedBy must be "frantz-kati" for published pages, got "${meta.reviewedBy}"`);
  }
  validateMetaDates(path, meta, errors);
  validateMetaRivals(path, meta, errors);
  validateMetaProof(path, meta, siteDir, errors);
}

type RouteLike = Pick<Route, "path" | "title" | "description" | "meta">;

function validateSingleRoute(
  route: RouteLike,
  seenPaths: Set<string>,
  seenTitles: Map<string, string>,
  seenDescriptions: Map<string, string>,
  siteDir: string,
  errors: string[],
): void {
  if (!route.path.startsWith("/") || (route.path.length > 1 && route.path.endsWith("/"))) {
    errors.push(`${route.path}: path must start with "/" and omit trailing slash`);
  }
  if (seenPaths.has(route.path)) {
    errors.push(`${route.path}: duplicate route path`);
  }
  seenPaths.add(route.path);

  const title = route.title.trim();
  if (!title || title.length > 60) {
    errors.push(`${route.path}: title length must be 1 to 60 chars, got ${title.length}`);
  }
  const priorTitle = seenTitles.get(title);
  if (priorTitle) {
    errors.push(`${route.path}: duplicate title also used on ${priorTitle}`);
  }
  seenTitles.set(title, route.path);

  const description = route.description.trim();
  if (!description || description.length > 160) {
    errors.push(`${route.path}: description length must be 1 to 160 chars, got ${description.length}`);
  }
  const priorDesc = seenDescriptions.get(description);
  if (priorDesc) {
    errors.push(`${route.path}: duplicate description also used on ${priorDesc}`);
  }
  seenDescriptions.set(description, route.path);

  if (route.meta) {
    validateMeta(route.path, route.meta, siteDir, errors);
  }
}

function validateRouteList(list: RouteLike[], siteDir = SITE_DIR): string[] {
  const errors: string[] = [];
  const seenPaths = new Set<string>();
  const seenTitles = new Map<string, string>();
  const seenDescriptions = new Map<string, string>();

  for (const route of list) {
    validateSingleRoute(route, seenPaths, seenTitles, seenDescriptions, siteDir, errors);
  }

  return errors;
}

describe("site routes", () => {
  it("keeps every registered route valid, unique, and within SEO character limits", () => {
    expect(validateRouteList(routes)).toEqual([]);
  });

  it("accepts a valid published PageMeta entry with existing proof and rival sources", () => {
    const sample: RouteLike[] = [
      {
        path: "/vs/intercom",
        title: "dimmi vs Intercom: free seats & BYO AI keys",
        description:
          "Pick Intercom for SOC 2 and its app marketplace. Pick dimmi for free seats, BYO AI keys, MIT code, and 100 free conversations every month.",
        meta: {
          type: "vs",
          keyword: "intercom alternative",
          answer: "Pick Intercom for SOC 2. Pick dimmi for free seats and BYO AI keys.",
          author: "frantz-kati",
          published: "2026-11-27",
          updated: "2026-11-27",
          status: "published",
          reviewedBy: "frantz-kati",
          reviewedOn: "2026-11-27",
          batch: "wave-1",
          rivals: [
            {
              name: "Intercom",
              checkedOn: "2026-11-27",
              source: "https://www.intercom.com/pricing",
            },
          ],
          proof: ["og.png"],
        },
      },
    ];
    expect(validateRouteList(sample)).toEqual([]);
  });

  it("rejects non-frantz-kati reviewers, inverted dates, missing proof files, and duplicate metadata", () => {
    const sample: RouteLike[] = [
      {
        path: "/vs/sample-a",
        title: "A title that is intentionally much longer than sixty characters total",
        description: "Shared description used on two routes.",
        meta: {
          type: "vs",
          keyword: "sample alternative",
          answer: "Direct answer.",
          author: "frantz-kati",
          published: "2026-11-27",
          updated: "2026-11-20",
          status: "published",
          reviewedBy: "github-actions[bot]",
          reviewedOn: "2026-02-30",
          rivals: [{ name: "Sample", checkedOn: "2026-11-27", source: "not-a-url" }],
          proof: ["shots/does-not-exist.webp"],
        },
      },
      {
        path: "/vs/sample-b",
        title: "Valid short title",
        description: "Shared description used on two routes.",
      },
    ];

    const errors = validateRouteList(sample);
    expect(errors.some((e) => e.includes("title length"))).toBe(true);
    expect(errors.some((e) => e.includes("duplicate description"))).toBe(true);
    expect(errors.some((e) => e.includes('meta.reviewedBy must be "frantz-kati"'))).toBe(true);
    expect(errors.some((e) => e.includes("meta.reviewedOn"))).toBe(true);
    expect(errors.some((e) => e.includes("earlier than meta.published"))).toBe(true);
    expect(errors.some((e) => e.includes("invalid source URL"))).toBe(true);
    expect(errors.some((e) => e.includes("meta.proof file not found"))).toBe(true);
  });
});
