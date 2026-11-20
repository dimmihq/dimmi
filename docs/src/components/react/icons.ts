import * as Icons from "@remixicon/react";
import type { ComponentType } from "react";

export type IconComponent = ComponentType<{ className?: string; "aria-hidden"?: boolean | "true" | "false" }>;

/*
 * A Remix icon by name: "rocket-2" finds RiRocket2Line, then RiRocket2,
 * then RiRocket2Fill; a full name such as "RiGithubFill" is used as is.
 * Server-side only, so the browser never loads the whole icon set.
 */
export function icon(name: string | undefined): IconComponent | undefined {
  if (!name) {
    return undefined;
  }
  const set = Icons as unknown as Record<string, IconComponent>;
  if (name.startsWith("Ri")) {
    return set[name];
  }
  const base = `Ri${name.replace(/(^|-)(\w)/g, (_, __, c: string) => c.toUpperCase())}`;
  return /(Line|Fill)$/.test(base) ? set[base] : (set[`${base}Line`] ?? set[base] ?? set[`${base}Fill`]);
}
