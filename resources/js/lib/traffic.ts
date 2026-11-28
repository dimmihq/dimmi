/*
 * Marketing signup event counts for dimmi Cloud production. Loads Rustrak's
 * Web Traffic script lazily with data-auto-track="false" so the dashboard
 * sends zero automatic page views and only counts explicit signup milestones.
 * Self-hosted builds, staging, and local builds never load the script or send
 * anything. No emails, names, or person IDs are ever attached.
 */

type TrafficValue = string | number | boolean;
type TrafficData = Record<string, TrafficValue>;

const TRACKER_HOST = "https://errors.trydimmi.com";
const PROJECT_ID = "3";
const PROJECT_KEY = "ee0d3c1646fa87790f5f180d1d4ba547";
const PRODUCTION_HOST = "trydimmi.com";
const UTM_STORAGE_KEY = "dimmi_utm";
const UTM_KEYS = ["utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"] as const;
const MAX_UTM_VALUE_LENGTH = 200;

declare global {
  interface Window {
    webTraffic?: {
      track?: (name: string, data?: TrafficData) => void;
    };
  }
}

type PendingEvent = {
  name: string;
  data?: TrafficData;
};

let scriptLoading = false;
const pending: PendingEvent[] = [];

/** Whether the browser is running dimmi Cloud in production. */
export function isCloudProduction(edition?: string, hostname?: string): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  const host = hostname ?? window.location.hostname;
  return edition === "cloud" && host === PRODUCTION_HOST;
}

function dispatch(name: string, data?: TrafficData): boolean {
  if (typeof window === "undefined" || typeof window.webTraffic?.track !== "function") {
    return false;
  }
  try {
    if (data !== undefined) {
      window.webTraffic.track(name, data);
    } else {
      window.webTraffic.track(name);
    }
    return true;
  } catch {
    return false;
  }
}

function ensureTrafficScript(doc: Document = document): void {
  if (typeof window.webTraffic?.track === "function") {
    return;
  }
  if (doc.querySelector('script[data-dimmi-web-traffic="true"]')) {
    return;
  }

  scriptLoading = true;
  const script = doc.createElement("script");
  script.defer = true;
  script.src = `${TRACKER_HOST}/public/web-traffic/script.js`;
  script.dataset.project = PROJECT_ID;
  script.dataset.key = PROJECT_KEY;
  script.dataset.autoTrack = "false";
  script.dataset.dimmiWebTraffic = "true";

  const finish = (flush: boolean) => {
    scriptLoading = false;
    const queued = pending.splice(0, pending.length);
    if (flush) {
      for (const item of queued) {
        dispatch(item.name, item.data);
      }
    }
  };
  script.addEventListener("load", () => finish(true), { once: true });
  script.addEventListener("error", () => finish(false), { once: true });
  doc.head.append(script);
}

function sendEvent(name: string, data?: TrafficData, doc: Document = document): void {
  ensureTrafficScript(doc);
  if (dispatch(name, data)) {
    return;
  }
  if (scriptLoading && pending.length < 16) {
    pending.push({ name, data });
  }
}

function readStoredUtmSource(): string | undefined {
  try {
    const raw = window.sessionStorage.getItem(UTM_STORAGE_KEY);
    if (!raw) {
      return undefined;
    }
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      return undefined;
    }
    const source = (parsed as Record<string, unknown>).utm_source;
    if (typeof source === "string" && source.trim()) {
      return source.trim().slice(0, MAX_UTM_VALUE_LENGTH);
    }
  } catch {
    return undefined;
  }
  return undefined;
}

/** Reads first-touch `utm_source` from sessionStorage or the current URL query string. */
export function readUtmSource(search?: string): string | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const stored = readStoredUtmSource();
  if (stored) {
    return stored;
  }

  const params = new URLSearchParams(search ?? window.location.search);
  const incoming: Record<string, string> = {};
  for (const key of UTM_KEYS) {
    const value = params.get(key)?.trim();
    if (value) {
      incoming[key] = value.slice(0, MAX_UTM_VALUE_LENGTH);
    }
  }
  if (Object.keys(incoming).length > 0) {
    try {
      if (!window.sessionStorage.getItem(UTM_STORAGE_KEY)) {
        window.sessionStorage.setItem(UTM_STORAGE_KEY, JSON.stringify(incoming));
      }
    } catch {
      // Ignore blocked storage.
    }
  }
  return incoming.utm_source;
}

/** Counts when `/signup` is shown on dimmi Cloud production. */
export function trackSignupStarted(
  edition?: string,
  options?: { hostname?: string; search?: string; doc?: Document },
): void {
  if (!isCloudProduction(edition, options?.hostname)) {
    return;
  }
  const utmSource = readUtmSource(options?.search);
  sendEvent("signup_started", utmSource ? { utm_source: utmSource } : undefined, options?.doc);
}

/** Counts when a workspace is created on dimmi Cloud production. */
export function trackSignupDone(edition?: string, options?: { hostname?: string; doc?: Document }): void {
  if (!isCloudProduction(edition, options?.hostname)) {
    return;
  }
  sendEvent("signup_done", { plan: "cloud" }, options?.doc);
}

/** Resets internal script-loading state (for tests). */
export function resetTrafficState(): void {
  scriptLoading = false;
  pending.length = 0;
}
