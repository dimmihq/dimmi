/*
 * Applies a rival's brand accent as `--rival` on a comparison page wrapper.
 * dimmi keeps its emerald accent everywhere.
 */
import type { CSSProperties } from "react";

import type { Rival } from "../data/rivals";

export function rivalToneStyle(rival: Pick<Rival, "tone">): CSSProperties {
  return { "--rival": rival.tone } as CSSProperties;
}

export function formatIsoDate(iso: string): string {
  const [year, month, day] = iso.split("-").map(Number);
  if (!year || !month || !day) {
    return iso;
  }
  return new Date(Date.UTC(year, month - 1, day)).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}
