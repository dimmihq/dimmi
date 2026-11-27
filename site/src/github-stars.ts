/*
 * Star count formatting, threshold gating, and accessible labels shared by the
 * GitHubStars component and unit tests. Kept separate from the JSX module so
 * Fast Refresh only sees component exports in github-stars.tsx.
 */

declare const __GITHUB_STARS__: number | null;

/** Minimum star count required before the number renders beside the button. */
export const STAR_COUNT_THRESHOLD = 100;

/** Star count injected by Vite at build time, or null when unavailable. */
export const BUILD_GITHUB_STARS: number | null =
  typeof __GITHUB_STARS__ === "number" && Number.isFinite(__GITHUB_STARS__) && __GITHUB_STARS__ >= 0
    ? Math.floor(__GITHUB_STARS__)
    : null;

export function shouldShowStarCount(count: number | null, threshold = STAR_COUNT_THRESHOLD): count is number {
  return count !== null && Number.isFinite(count) && count >= threshold;
}

export function formatStarCount(count: number): string {
  const safe = Math.max(0, Math.floor(count));
  if (safe < 1_000) {
    return String(safe);
  }
  if (safe < 999_500) {
    const thousands = safe / 1_000;
    const rounded = thousands >= 100 ? Math.round(thousands) : Math.round(thousands * 10) / 10;
    return `${rounded % 1 === 0 ? rounded.toFixed(0) : rounded.toFixed(1)}k`;
  }
  const millions = safe / 1_000_000;
  const rounded = millions >= 100 ? Math.round(millions) : Math.round(millions * 10) / 10;
  return `${rounded % 1 === 0 ? rounded.toFixed(0) : rounded.toFixed(1)}m`;
}

export function starButtonLabel(count: number | null, threshold = STAR_COUNT_THRESHOLD): string {
  if (shouldShowStarCount(count, threshold)) {
    return `Star dimmi on GitHub, ${count.toLocaleString("en-US")} stars`;
  }
  return "Star dimmi on GitHub";
}
