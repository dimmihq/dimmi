/*
 * The domains a teammate chose to stop being asked about when they open a
 * link from a customer's message, kept in this browser for each
 * workspace. Storage can be missing or refuse, so every read and write
 * gives up quietly and the teammate is asked again.
 */
import { useSyncExternalStore } from "react";

import { under } from "./link-checks";

const listeners = new Set<() => void>();
const empty: string[] = [];
const cache = new Map<string, { raw: string | null; list: string[] }>();

function keyOf(workspace: string): string {
  return `dimmi:links:trusted:${workspace}`;
}

function read(workspace: string): string[] {
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(keyOf(workspace));
  } catch {
    return empty;
  }
  const hit = cache.get(workspace);
  if (hit && hit.raw === raw) {
    return hit.list;
  }
  let list = empty;
  try {
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    list = Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string") : empty;
  } catch {
    list = empty;
  }
  cache.set(workspace, { raw, list });

  return list;
}

/*
 * trustDomain stops the asking about links to the domain and the names
 * under it.
 */
export function trustDomain(workspace: string, domain: string): void {
  const now = read(workspace);
  if (now.includes(domain)) {
    return;
  }
  try {
    window.localStorage.setItem(keyOf(workspace), JSON.stringify([...now, domain]));
  } catch {
    return;
  }
  for (const listener of listeners) {
    listener();
  }
}

/*
 * isTrusted reports whether the host is under a trusted domain.
 */
export function isTrusted(trusted: string[], host: string): boolean {
  return trusted.some((domain) => under(host, domain));
}

/*
 * useTrustedDomains returns the workspace's trusted domains and follows
 * changes made anywhere in the app.
 */
export function useTrustedDomains(workspace: string): string[] {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);

      return () => listeners.delete(listener);
    },
    () => read(workspace),
    () => empty,
  );
}
